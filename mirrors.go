package main

import (
	_ "embed"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
	"strings"
)

// legacyMirrorsHandler will list the mirrors by region in the legacy format
// it is preferred to use mirrors.json, but this handler is here for build support
func legacyMirrorsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	mirrorOutput := make(map[string][]string)

	for region, mirrors := range regionMap {
		list := make([]string, len(mirrors))

		for i, mirror := range mirrors {
			list[i] = r.URL.Scheme + "://" + mirror.Host + "/" + strings.TrimLeft(mirror.Path, "/")
		}

		mirrorOutput[region] = list
	}

	json.NewEncoder(w).Encode(mirrorOutput)
}

// mirrorsHandler is a simple handler that will return the list of servers
func mirrorsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(servers)
}

var (
	//go:embed assets/status-up.svg
	statusUp []byte

	//go:embed assets/status-down.svg
	statusDown []byte

	//go:embed assets/status-unknown.svg
	statusUnknown []byte
)

// mirrorStatusHandler is a fancy svg-returning handler.
// it is used to display mirror statuses on a config repo of sorts
func mirrorStatusHandler(w http.ResponseWriter, r *http.Request) {
	serverHost := chi.URLParam(r, "server")

	w.Header().Set("Content-Type", "image/svg+xml;charset=utf-8")
	w.Header().Set("Cache-Control", "max-age=120")

	if serverHost == "" {
		w.Write(statusUnknown)
		return
	}

	serverHost = strings.Replace(serverHost, "_", ".", -1)

	server, ok := hostMap[serverHost]

	if !ok {
		w.Header().Set("Content-Length", strconv.Itoa(len(statusUnknown)))
		w.Write(statusUnknown)
		return
	}

	key := "offline"

	if server.Available {
		key = "online"
	}

	w.Header().Set("ETag", "\""+key+"\"")

	if match := r.Header.Get("If-None-Match"); match != "" {
		if strings.Trim(match, "\"") == key {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	if server.Available {
		w.Header().Set("Content-Length", strconv.Itoa(len(statusUp)))
		w.Write(statusUp)
	} else {
		w.Header().Set("Content-Length", strconv.Itoa(len(statusDown)))
		w.Write(statusDown)
	}
}
