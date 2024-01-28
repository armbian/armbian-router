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
	go r.servers.checkLoop(r, r.checks)

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
