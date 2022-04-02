package main

import (
	_ "embed"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strings"
)

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

func mirrorStatusHandler(w http.ResponseWriter, r *http.Request) {
	serverHost := chi.URLParam(r, "server")

	w.Header().Set("Content-Type", "image/svg+xml;charset=utf-8")

	if serverHost == "" {
		w.Write(statusUnknown)
		return
	}

	serverHost = strings.Replace(serverHost, "_", ".", -1)

	server, ok := hostMap[serverHost]

	if !ok {
		w.Write(statusUnknown)
		return
	}

	if server.Available {
		w.Write(statusUp)
	} else {
		w.Write(statusDown)
	}
}
