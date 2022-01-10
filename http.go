package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
)

func statusRequest(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func redirectRequest(w http.ResponseWriter, r *http.Request) {
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

	u := &url.URL{
		Scheme: scheme,
		Host:   server.Host,
		Path:   path.Join(server.Path, r.URL.Path),
	}

	w.Header().Set("X-Geo-Distance", fmt.Sprintf("%f", distance))
	w.Header().Set("Location", u.String())
	w.WriteHeader(http.StatusFound)
}
