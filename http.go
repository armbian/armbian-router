package main

import (
	"encoding/json"
	"fmt"
	"github.com/jmcvetta/randutil"
	"github.com/spf13/viper"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

// statusHandler is a simple handler that will always return 200 OK with a body of "OK"
func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	if r.Method != http.MethodHead {
		w.Write([]byte("OK"))
	}
}

// redirectHandler is the default "not found" handler which handles redirects
// if the environment variable OVERRIDE_IP is set, it will use that ip address
// this is useful for local testing when you're on the local network
func redirectHandler(w http.ResponseWriter, r *http.Request) {
	ipStr, _, err := net.SplitHostPort(r.RemoteAddr)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ip := net.ParseIP(ipStr)

	if ip.IsLoopback() || ip.IsPrivate() {
		overrideIP := os.Getenv("OVERRIDE_IP")

		if overrideIP == "" {
			overrideIP = "1.1.1.1"
		}

		ip = net.ParseIP(overrideIP)
	}

	var server *Server
	var distance float64

	// If the path has a prefix of region/NA, it will use specific regions instead
	// of the default geographical distance
	if strings.HasPrefix(r.URL.Path, "/region") {
		parts := strings.Split(r.URL.Path, "/")

		// region = parts[2]
		if mirrors, ok := regionMap[parts[2]]; ok {
			choices := make([]randutil.Choice, len(mirrors))

			for i, item := range mirrors {
				if !item.Available {
					continue
				}

				choices[i] = randutil.Choice{
					Weight: item.Weight,
					Item:   item,
				}
			}

			choice, err := randutil.WeightedChoice(choices)

			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			server = choice.Item.(*Server)

			r.URL.Path = strings.Join(parts[3:], "/")
		}
	}

	// If none of the above exceptions are matched, we use the geographical distance based on IP
	if server == nil {
		server, distance, err = servers.Closest(ip)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// If we don't have a scheme, we'll use https by default
	scheme := r.URL.Scheme

	if scheme == "" {
		scheme = "https"
	}

	// redirectPath is a combination of server path (which can be something like /armbian)
	// and the URL path.
	// Example: /armbian + /some/path = /armbian/some/path
	redirectPath := path.Join(server.Path, r.URL.Path)

	// If we have a dlMap, we map the url to a final path instead
	if dlMap != nil {
		if newPath, exists := dlMap[strings.TrimLeft(r.URL.Path, "/")]; exists {
			downloadsMapped.Inc()
			redirectPath = path.Join(server.Path, newPath)
		}
	}

	if strings.HasSuffix(r.URL.Path, "/") && !strings.HasSuffix(redirectPath, "/") {
		redirectPath += "/"
	}

	// We need to build the final url now
	u := &url.URL{
		Scheme: scheme,
		Host:   server.Host,
		Path:   redirectPath,
	}

	server.Redirects.Inc()
	redirectsServed.Inc()

	// If we used geographical distance, we add an X-Geo-Distance header for debug.
	if distance > 0 {
		w.Header().Set("X-Geo-Distance", fmt.Sprintf("%f", distance))
	}

	w.Header().Set("Location", u.String())
	w.WriteHeader(http.StatusFound)
}

// reloadHandler is an http handler which lets us reload the server configuration
// It is only enabled when the reloadToken is set in the configuration
func reloadHandler(w http.ResponseWriter, r *http.Request) {
	expectedToken := viper.GetString("reloadToken")

	if expectedToken == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	token := r.Header.Get("Authorization")

	if token == "" || !strings.HasPrefix(token, "Bearer") || !strings.Contains(token, " ") {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	token = token[strings.Index(token, " ")+1:]

	if token != expectedToken {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if err := reloadConfig(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func dlMapHandler(w http.ResponseWriter, r *http.Request) {
	if dlMap == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(dlMap)
}

func geoIPHandler(w http.ResponseWriter, r *http.Request) {
	ipStr, _, err := net.SplitHostPort(r.RemoteAddr)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ip := net.ParseIP(ipStr)

	var city City
	err = db.Lookup(ip, &city)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(city)
}
