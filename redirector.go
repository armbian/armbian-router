package redirector

import (
	"github.com/armbian/redirector/middleware"
	"github.com/chi-middleware/logrus-logger"
	"github.com/go-chi/chi/v5"
	lru "github.com/hashicorp/golang-lru"
	"github.com/oschwald/maxminddb-golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
)

var (
	redirectsServed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "armbian_router_redirects",
		Help: "The total number of processed redirects",
	})

	downloadsMapped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "armbian_router_download_maps",
		Help: "The total number of mapped download paths",
	})
)

// Redirector is our application instance.
type Redirector struct {
	config      *Config
	db          *maxminddb.Reader
	asnDB       *maxminddb.Reader
	servers     ServerList
	regionMap   map[string][]*Server
	hostMap     map[string]*Server
	dlMap       map[string]string
	topChoices  int
	serverCache *lru.Cache
	checks      []ServerCheck
	checkClient *http.Client
}

// City represents a MaxmindDB city.
// This used to only be used on load, but is now used with rules as well.
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
		AccuracyRadius uint16  `maxminddb:"accuracy_radius" json:"accuracy_radius"`
		Latitude       float64 `maxminddb:"latitude" json:"latitude"`
		Longitude      float64 `maxminddb:"longitude" json:"longitude"`
	} `maxminddb:"location"`
	RegisteredCountry struct {
		GeoNameID uint              `maxminddb:"geoname_id" json:"geoname_id"`
		IsoCode   string            `maxminddb:"iso_code" json:"iso_code"`
		Names     map[string]string `maxminddb:"names" json:"names"`
	} `maxminddb:"registered_country" json:"registered_country"`
}

// The ASN struct corresponds to the data in the GeoLite2 ASN database.
type ASN struct {
	AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}

// ServerConfig is a configuration struct holding basic server configuration.
// This is used for initial loading of server information before parsing into Server.
type ServerConfig struct {
	Server    string   `mapstructure:"server" yaml:"server"`
	Latitude  float64  `mapstructure:"latitude" yaml:"latitude"`
	Longitude float64  `mapstructure:"longitude" yaml:"longitude"`
	Continent string   `mapstructure:"continent"`
	Weight    int      `mapstructure:"weight" yaml:"weight"`
	Protocols []string `mapstructure:"protocols" yaml:"protocols"`
	Rules     []Rule   `mapstructure:"rules" yaml:"rules"`
}

// Rule defines a matching rule on a server.
// This can be used to exclude ASNs, Countries, and more from a server.
type Rule struct {
	Field string   `mapstructure:"field" yaml:"field" json:"field"`
	Is    string   `mapstructure:"is" yaml:"is" json:"is,omitempty"`
	IsNot string   `mapstructure:"is_not" yaml:"is_not" json:"is_not,omitempty"`
	In    []string `mapstructure:"in" yaml:"in" json:"in,omitempty"`
	NotIn []string `mapstructure:"not_in" yaml:"not_in" json:"not_in,omitempty"`
}

// New creates a new instance of Redirector
func New(config *Config) *Redirector {
	r := &Redirector{
		config: config,
	}

	r.checks = []ServerCheck{
		&HTTPCheck{
			config: config,
		},
		&TLSCheck{
			config: config,
		},
	}

	if config.CheckURL != "" {
		r.checks = append(r.checks, &VersionCheck{
			config:     config,
			VersionURL: config.CheckURL,
		})
	}

	return r
}

// Start registers the routes, loggers, and server checks,
// then returns the http.Handler.
func (r *Redirector) Start() http.Handler {
	if err := r.ReloadConfig(); err != nil {
		log.WithError(err).Fatalln("Unable to load configuration")
	}

	log.Info("Starting check loop")

	// Start check loop
	go r.servers.checkLoop(r.checks)

	log.Info("Setting up routes")

	router := chi.NewRouter()

	router.Use(middleware.RealIPMiddleware)
	router.Use(logger.Logger("router", log.StandardLogger()))

	router.Head("/status", r.statusHandler)
	router.Get("/status", r.statusHandler)
	router.Get("/mirrors", r.legacyMirrorsHandler)
	router.Get("/mirrors/{server}.svg", r.mirrorStatusHandler)
	router.Get("/mirrors.json", r.mirrorsHandler)
	router.Post("/reload", r.reloadHandler)
	router.Get("/dl_map", r.dlMapHandler)
	router.Get("/geoip", r.geoIPHandler)
	router.Get("/metrics", promhttp.Handler().ServeHTTP)

	router.NotFound(r.redirectHandler)

	if r.config.BindAddress != "" {
		log.WithField("bind", r.config.BindAddress).Info("Binding to address")

		go http.ListenAndServe(r.config.BindAddress, router)
	}

	return router
}
