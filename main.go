package main

import (
	"flag"
	"github.com/chi-middleware/logrus-logger"
	"github.com/go-chi/chi/v5"
	lru "github.com/hashicorp/golang-lru"
	"github.com/oschwald/maxminddb-golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

var (
	db         *maxminddb.Reader
	servers    ServerList
	regionMap  map[string][]*Server
	hostMap    map[string]*Server
	dlMap      map[string]string
	topChoices int

	redirectsServed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "armbian_router_redirects",
		Help: "The total number of processed redirects",
	})

	downloadsMapped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "armbian_router_download_maps",
		Help: "The total number of mapped download paths",
	})

	serverCache *lru.Cache
)

type LocationLookup struct {
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
}

// City represents a MaxmindDB city
type City struct {
	Continent struct {
		Code      string            `maxminddb:"code" json:"code"`
		GeoNameID uint              `maxminddb:"geoname_id" json:"geoname_id"`
		Names     map[string]string `maxminddb:"names" json:"names"`
	} `maxminddb:"continent" json:"continent"`
	Country struct {
		GeoNameID uint              `maxminddb:"geoname_id" json:"geoname_id"`
		IsoCode   string            `maxminddb:"iso_code" json:"iso_code"`
		Names     map[string]string `maxminddb:"names" json:"names"`
	} `maxminddb:"country" json:"country"`
	Location struct {
		AccuracyRadius uint16  `maxminddb:"accuracy_radius" json:'accuracy_radius'`
		Latitude       float64 `maxminddb:"latitude" json:"latitude"`
		Longitude      float64 `maxminddb:"longitude" json:"longitude"`
	} `maxminddb:"location"`
	RegisteredCountry struct {
		GeoNameID uint              `maxminddb:"geoname_id" json:"geoname_id"`
		IsoCode   string            `maxminddb:"iso_code" json:"iso_code"`
		Names     map[string]string `maxminddb:"names" json:"names"`
	} `maxminddb:"registered_country" json:"registered_country"`
}

type ServerConfig struct {
	Server    string  `mapstructure:"server" yaml:"server"`
	Latitude  float64 `mapstructure:"latitude" yaml:"latitude"`
	Longitude float64 `mapstructure:"longitude" yaml:"longitude"`
	Continent string  `mapstructure:"continent"`
	Weight    int     `mapstructure:"weight" yaml:"weight"`
}

var (
	configFlag = flag.String("config", "", "configuration file path")
)

func main() {
	flag.Parse()

	viper.SetDefault("bind", ":8080")
	viper.SetDefault("cacheSize", 1024)
	viper.SetDefault("topChoices", 3)

	viper.SetConfigName("dlrouter")        // name of config file (without extension)
	viper.SetConfigType("yaml")            // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath("/etc/dlrouter/")  // path to look for the config file in
	viper.AddConfigPath("$HOME/.dlrouter") // call multiple times to add many search paths
	viper.AddConfigPath(".")               // optionally look for config in the working directory

	if *configFlag != "" {
		viper.SetConfigFile(*configFlag)
	}

	reloadConfig()

	// Start check loop
	go servers.checkLoop()

	log.Info("Starting")

	r := chi.NewRouter()

	r.Use(RealIPMiddleware)
	r.Use(logger.Logger("router", log.StandardLogger()))

	r.Head("/status", statusHandler)
	r.Get("/status", statusHandler)
	r.Get("/mirrors", legacyMirrorsHandler)
	r.Get("/mirrors/{server}.svg", mirrorStatusHandler)
	r.Get("/mirrors.json", mirrorsHandler)
	r.Post("/reload", reloadHandler)
	r.Get("/dl_map", dlMapHandler)
	r.Get("/geoip", geoIPHandler)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	r.NotFound(redirectHandler)

	go http.ListenAndServe(viper.GetString("bind"), r)

	log.Info("Ready")

	c := make(chan os.Signal)

	signal.Notify(c, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGHUP)

	for {
		sig := <-c

		if sig != syscall.SIGHUP {
			break
		}

		reloadConfig()
	}
}
