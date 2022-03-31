package main

import (
	"flag"
	"github.com/chi-middleware/logrus-logger"
	"github.com/go-chi/chi/v5"
	"github.com/oschwald/maxminddb-golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

var (
	db      *maxminddb.Reader
	servers ServerList

	dlMap map[string]string

	redirectsServed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "armbian_router_redirects",
		Help: "The total number of processed redirects",
	})

	downloadsMapped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "armbian_router_download_maps",
		Help: "The total number of mapped download paths",
	})
)

// City represents a MaxmindDB city
type City struct {
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
}

var (
	configFlag = flag.String("config", "", "configuration file path")
)

func main() {
	flag.Parse()

	viper.SetDefault("bind", ":8080")

	viper.SetConfigName("dlrouter")        // name of config file (without extension)
	viper.SetConfigType("yaml")            // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath("/etc/dlrouter/")  // path to look for the config file in
	viper.AddConfigPath("$HOME/.dlrouter") // call multiple times to add many search paths
	viper.AddConfigPath(".")               // optionally look for config in the working directory

	if *configFlag != "" {
		viper.SetConfigFile(*configFlag)
	}

	err := viper.ReadInConfig() // Find and read the config file

	if err != nil { // Handle errors reading the config file
		log.WithError(err).Fatalln("Unable to load config file")
	}

	db, err = maxminddb.Open(viper.GetString("geodb"))

	if err != nil {
		log.WithError(err).Fatalln("Unable to open database")
	}

	if mapFile := viper.GetString("dl_map"); mapFile != "" {
		log.WithField("file", mapFile).Info("Loading download map")

		dlMap, err = loadMap(mapFile)

		if err != nil {
			log.WithError(err).Fatalln("Unable to load download map")
		}
	}

	serverList := viper.GetStringSlice("servers")

	var wg sync.WaitGroup

	for _, server := range serverList {
		wg.Add(1)

		go func(server string) {
			defer wg.Done()

			addServer(server)
		}(server)
	}

	wg.Wait()

	log.Info("Servers added, checking statuses")

	// Start check loop
	go servers.checkLoop()

	log.Info("Starting")

	r := chi.NewRouter()

	r.Use(RealIPMiddleware)
	r.Use(logger.Logger("router", log.StandardLogger()))

	r.Get("/status", statusHandler)
	r.Get("/mirrors", mirrorsHandler)
	r.Post("/reload", reloadHandler)
	r.Get("/dl_map", dlMapHandler)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	r.NotFound(redirectHandler)

	go http.ListenAndServe(viper.GetString("bind"), r)

	c := make(chan os.Signal)

	signal.Notify(c, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGHUP)

	for {
		sig := <-c

		if sig != syscall.SIGHUP {
			break
		}

		reloadMap()
	}
}

var metricReplacer = strings.NewReplacer(".", "_", "-", "_")

func addServer(server string) {
	var prefix string

	if !strings.HasPrefix(server, "http") {
		prefix = "https://"
	}

	u, err := url.Parse(prefix + server)

	if err != nil {
		log.WithFields(log.Fields{
			"error":  err,
			"server": server,
		}).Warning("Server is invalid")
		return
	}

	ips, err := net.LookupIP(u.Host)

	if err != nil {
		log.WithFields(log.Fields{
			"error":  err,
			"server": server,
		}).Warning("Could not resolve address")
		return
	}

	var city City
	err = db.Lookup(ips[0], &city)

	if err != nil {
		log.WithFields(log.Fields{
			"error":  err,
			"server": server,
		}).Warning("Could not geolocate address")
		return
	}

	redirects := promauto.NewCounter(prometheus.CounterOpts{
		Name: "armbian_router_redirects_" + metricReplacer.Replace(u.Host),
		Help: "The number of redirects for server " + u.Host,
	})

	servers = append(servers, &Server{
		Host:      u.Host,
		Path:      u.Path,
		Latitude:  city.Location.Latitude,
		Longitude: city.Location.Longitude,
		Redirects: redirects,
	})

	log.WithFields(log.Fields{
		"server":    u.Host,
		"path":      u.Path,
		"latitude":  city.Location.Latitude,
		"longitude": city.Location.Longitude,
	}).Info("Added server")
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
