package main

import (
	lru "github.com/hashicorp/golang-lru"
	"github.com/oschwald/maxminddb-golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"net"
	"net/url"
	"strings"
	"sync"
)

func reloadConfig() {
	err := viper.ReadInConfig() // Find and read the config file

	if err != nil { // Handle errors reading the config file
		log.WithError(err).Fatalln("Unable to load config file")
	}

	// db will never be reloaded.
	if db == nil {
		// Load maxmind database
		db, err = maxminddb.Open(viper.GetString("geodb"))

		if err != nil {
			log.WithError(err).Fatalln("Unable to open database")
		}
	}

	// Refresh server cache if size changed
	if serverCache == nil {
		serverCache, err = lru.New(viper.GetInt("cacheSize"))
	} else {
		serverCache.Resize(viper.GetInt("cacheSize"))
	}

	// Set top choice count
	topChoices = viper.GetInt("topChoices")

	// Reload map file
	reloadMap()

	// Reload server list
	reloadServers()

	// Check top choices size
	if topChoices > len(servers) {
		topChoices = len(servers)
	}

	// Force check
	go servers.Check()
}

func reloadServers() {
	var serverList []ServerConfig
	viper.UnmarshalKey("servers", &serverList)

	var wg sync.WaitGroup

	existing := make(map[string]int)

	for i, server := range servers {
		existing[server.Host] = i
	}

	hosts := make(map[string]bool)

	for _, server := range serverList {
		wg.Add(1)

		var prefix string

		if !strings.HasPrefix(server.Server, "http") {
			prefix = "https://"
		}

		u, err := url.Parse(prefix + server.Server)

		if err != nil {
			log.WithFields(log.Fields{
				"error":  err,
				"server": server,
			}).Warning("Server is invalid")
			return
		}

		hosts[u.Host] = true

		i := -1

		if v, exists := existing[u.Host]; exists {
			i = v
		}

		go func(i int, server ServerConfig, u *url.URL) {
			defer wg.Done()

			s := addServer(server, u)

			if _, ok := existing[u.Host]; ok {
				s.Redirects = servers[i].Redirects

				servers[i] = s
			} else {
				s.Redirects = promauto.NewCounter(prometheus.CounterOpts{
					Name: "armbian_router_redirects_" + metricReplacer.Replace(u.Host),
					Help: "The number of redirects for server " + u.Host,
				})

				servers = append(servers, s)

				log.WithFields(log.Fields{
					"server":    u.Host,
					"path":      u.Path,
					"latitude":  s.Latitude,
					"longitude": s.Longitude,
				}).Info("Added server")
			}
		}(i, server, u)
	}

	wg.Wait()

	// Remove servers that no longer exist in the config
	for i := len(servers) - 1; i >= 0; i-- {
		if _, exists := hosts[servers[i].Host]; exists {
			continue
		}

		log.WithFields(log.Fields{
			"server": servers[i].Host,
		}).Info("Removed server")

		servers = append(servers[:i], servers[i+1:]...)
	}
}

var metricReplacer = strings.NewReplacer(".", "_", "-", "_")

// addServer takes ServerConfig and constructs a server.
// This will create duplicate servers, but it will overwrite existing ones when changed.
func addServer(server ServerConfig, u *url.URL) *Server {
	s := &Server{
		Available: true,
		Host:      u.Host,
		Path:      u.Path,
		Latitude:  server.Latitude,
		Longitude: server.Longitude,
		Weight:    server.Weight,
	}

	if s.Weight == 0 {
		s.Weight = 1
	}

	if s.Latitude == 0 && s.Longitude == 0 {
		ips, err := net.LookupIP(u.Host)

		if err != nil {
			log.WithFields(log.Fields{
				"error":  err,
				"server": s.Host,
			}).Warning("Could not resolve address")
			return nil
		}

		var city City
		err = db.Lookup(ips[0], &city)

		if err != nil {
			log.WithFields(log.Fields{
				"error":  err,
				"server": s.Host,
				"ip":     ips[0],
			}).Warning("Could not geolocate address")
			return nil
		}

		s.Latitude = city.Location.Latitude
		s.Longitude = city.Location.Longitude
	}

	return s
}

func reloadMap() {
	mapFile := viper.GetString("dl_map")

	if mapFile == "" {
		return
	}

	log.WithField("file", mapFile).Info("Loading download map")

	newMap, err := loadMap(mapFile)

	if err != nil {
		return
	}

	dlMap = newMap
}
