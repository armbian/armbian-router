package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
)

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func mirrorsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(servers)
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	ipStr, _, err := net.SplitHostPort(r.RemoteAddr)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ip := net.ParseIP(ipStr)

	// TODO: This is temporary to allow testing on private addresses.
	if ip.IsPrivate() {
		ip = net.ParseIP("1.1.1.1")
	}

	server, distance, err := servers.Closest(ip)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	scheme := r.URL.Scheme

	if scheme == "" {
		scheme = "https"
	}

	redirectPath := path.Join(server.Path, r.URL.Path)

	if dlMap != nil {
		if newPath, exists := dlMap[strings.TrimLeft(r.URL.Path, "/")]; exists {
			downloadsMapped.Inc()
			redirectPath = path.Join(server.Path, newPath)
		}
	}

	u := &url.URL{
		Scheme: scheme,
		Host:   server.Host,
		Path:   redirectPath,
	}

	server.Redirects.Inc()
	redirectsServed.Inc()

	w.Header().Set("X-Geo-Distance", fmt.Sprintf("%f", distance))
	w.Header().Set("Location", u.String())
	w.WriteHeader(http.StatusFound)
}

func reloadHandler(w http.ResponseWriter, r *http.Request) {
	reloadMap()

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
