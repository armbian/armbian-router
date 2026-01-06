package redirector

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/armbian/redirector/db"
	lru "github.com/hashicorp/golang-lru"
	"github.com/oschwald/maxminddb-golang"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
)

// Config represents our application's configuration.
type Config struct {
	// BindAddress is the address to bind our webserver to.
	BindAddress string `mapstructure:"bind"`

	// GeoDBPath is the path to the MaxMind GeoLite2 City DB.
	GeoDBPath string `mapstructure:"geodb"`

	// CertDataPath is the path to fetch CA certs from system.
	// If empty, CAs will be fetched from Mozilla directly.
	CertDataPath string `mapstructure:"certDataPath"`

	// ASNDBPath is the path to the GeoLite2 ASN DB.
	ASNDBPath string `mapstructure:"asndb"`

	// MapFile is a file used to map download urls via redirect.
	MapFile string `mapstructure:"dl_map"`

	// CacheSize is the number of items to keep in the LRU cache.
	CacheSize int `mapstructure:"cacheSize"`

	// TopChoices is the number of servers to use in a rotation.
	// With the default being 3, the top 3 servers will be rotated based on weight.
	TopChoices int `mapstructure:"topChoices"`

	// ReloadToken is a secret token used for web-based reload.
	ReloadToken string `mapstructure:"reloadToken"`

	// CheckURL is the url used to verify mirror versions
	CheckURL string `mapstructure:"checkUrl"`

	// SameCityThreshold is the parameter used to specify a threshold between mirrors and the client
	SameCityThreshold float64 `mapstructure:"sameCityThreshold"`

	// ServerList is a list of ServerConfig structs, which gets parsed into servers.
	ServerList []ServerConfig `mapstructure:"servers"`

	// Special extensions for the download map
	SpecialExtensions map[string]string `mapstructure:"specialExtensions"`

	// LogLevel is the log level to use for the application.
	// It can be one of: "debug", "info", "warn", "error", "fatal", "panic".
	// If not set, it defaults to "warn".
	LogLevel string `mapstructure:"logLevel"`

	// EnableProfiler enables the pprof profiler on the server.
	// This is not recommended for production use.
	EnableProfiler bool `mapstructure:"enableProfiler"`

	// ReloadFunc is called when a reload is done via http api.
	ReloadFunc func()

	// RootCAs is a list of CA certificates, which we parse from Mozilla directly.
	RootCAs *x509.CertPool

	checkClient *http.Client
}

// SetRootCAs sets the root ca files, and creates the http client for checks
// This **MUST** be called before r.checkClient is used.
func (c *Config) SetRootCAs(cas *x509.CertPool) {
	c.RootCAs = cas

	t := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: cas,
		},
	}

	c.checkClient = &http.Client{
		Transport: t,
		Timeout:   20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func Remove[V comparable](collection []V, value V) []V {
	return lo.Filter(collection, func(item V, _ int) bool {
		return item != value
	})
}

// ReloadConfig is called to reload the server's configuration.
func (r *Redirector) ReloadConfig() error {
	log.Info("Loading configuration...")
	if reloadFunc := r.config.ReloadFunc; reloadFunc != nil {
		reloadFunc()
	}

	var err error

	// Load maxmind database
	if r.db != nil {
		err = r.db.Close()
		if err != nil {
			return errors.Wrap(err, "Unable to close database")
		}
	}

	if r.asnDB != nil {
		err = r.asnDB.Close()
		if err != nil {
			return errors.Wrap(err, "Unable to close asn database")
		}
	}

	// set log level
	level, err := log.ParseLevel(r.config.LogLevel)
	if err != nil {
		log.WithField("level", r.config.LogLevel).Warning("Invalid log level, using default")
		level = log.WarnLevel
	}

	log.SetLevel(level)

	// db can be hot-reloaded if the file changed
	r.db, err = maxminddb.Open(r.config.GeoDBPath)
	if err != nil {
		return errors.Wrap(err, "Unable to open database")
	}

	if r.config.ASNDBPath != "" {
		r.asnDB, err = maxminddb.Open(r.config.ASNDBPath)
		if err != nil {
			return errors.Wrap(err, "Unable to open asn database")
		}
	}

	// Refresh server cache if size changed
	if r.serverCache == nil {
		r.serverCache, err = lru.New(r.config.CacheSize)
	} else {
		r.serverCache.Resize(r.config.CacheSize)
	}

	// Purge the cache to ensure we don't have any invalid servers in it
	r.serverCache.Purge()

	// Reload map file
	if err := r.reloadMap(); err != nil {
		return errors.Wrap(err, "Unable to load map file")
	}

	// Reload server list
	if err := r.reloadServers(); err != nil {
		return errors.Wrap(err, "Unable to load servers")
	}

	// Create mirror map
	mirrors := make(map[string][]*Server)
	for _, server := range r.servers {
		mirrors[server.Continent] = append(mirrors[server.Continent], server)
	}
	mirrors["default"] = append(mirrors["NA"], mirrors["EU"]...)
	r.regionMap = mirrors

	hosts := make(map[string]*Server)
	for _, server := range r.servers {
		hosts[server.Host] = server
	}
	r.hostMap = hosts

	// Check top choices size
	if r.config.TopChoices == 0 {
		r.config.TopChoices = 3
	} else if r.config.TopChoices > len(r.servers) {
		r.config.TopChoices = len(r.servers)
	}

	// Check if on the config is declared or use default logic
	if r.config.SameCityThreshold == 0 {
		r.config.SameCityThreshold = 200000.0
	}

	// Force check
	go r.servers.Check(r, r.checks)

	return nil
}

func (r *Redirector) reloadServers() error {
	log.WithField("count", len(r.config.ServerList)).Info("Loading servers")
	var wg sync.WaitGroup
	var serversLock sync.Mutex

	existing := make(map[string]int)
	for i, server := range r.servers {
		existing[server.Host] = i
	}

	hosts := make(map[string]bool)
	var hostsLock sync.Mutex

	// Collect new servers to add after all goroutines complete
	type serverUpdate struct {
		index  int // -1 means new server
		server *Server
	}
	var updates []serverUpdate
	var updatesLock sync.Mutex

	for _, server := range r.config.ServerList {
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
			return err
		}

		i := -1
		if v, exists := existing[u.Host]; exists {
			i = v
		}

		go func(i int, server ServerConfig, u *url.URL) {
			defer wg.Done()
			s, err := r.addServer(server, u)
			if err != nil {
				log.WithError(err).Warning("Unable to add server")
				return
			}

			hostsLock.Lock()
			hosts[u.Host] = true
			hostsLock.Unlock()

			updatesLock.Lock()
			updates = append(updates, serverUpdate{index: i, server: s})
			updatesLock.Unlock()
		}(i, server, u)
	}

	wg.Wait()

	// Apply all updates after goroutines completed
	serversLock.Lock()
	for _, update := range updates {
		if update.index >= 0 && update.index < len(r.servers) {
			// Update existing server
			update.server.Redirects = r.servers[update.index].Redirects
			r.servers[update.index] = update.server
		} else if update.index == -1 {
			// Add new server
			update.server.Redirects = promauto.NewCounter(prometheus.CounterOpts{
				Name: "armbian_router_redirects_" + metricReplacer.Replace(update.server.Host),
				Help: "The number of redirects for server " + update.server.Host,
			})
			r.servers = append(r.servers, update.server)
			log.WithFields(log.Fields{
				"server":    update.server.Host,
				"path":      update.server.Path,
				"latitude":  update.server.Latitude,
				"longitude": update.server.Longitude,
				"country":   update.server.Country,
			}).Info("Added server")
		}
	}

	// Remove servers that no longer exist in the config
	for i := len(r.servers) - 1; i >= 0; i-- {
		if _, exists := hosts[r.servers[i].Host]; exists {
			continue
		}
		log.WithFields(log.Fields{
			"server": r.servers[i].Host,
		}).Info("Removed server")
		r.servers = append(r.servers[:i], r.servers[i+1:]...)
	}
	serversLock.Unlock()

	return nil
}

var metricReplacer = strings.NewReplacer(".", "_", "-", "_")

// addServer takes ServerConfig and constructs a server.
// This will create duplicate servers, but it will overwrite existing ones when changed.
func (r *Redirector) addServer(server ServerConfig, u *url.URL) (*Server, error) {
	s := &Server{
		Available: true,
		Host:      u.Host,
		Path:      u.Path,
		Latitude:  server.Latitude,
		Longitude: server.Longitude,
		Continent: server.Continent,
		Weight:    server.Weight,
		Protocols: []string{"http", "https"},
		Rules:     server.Rules,
	}
	if len(server.Protocols) > 0 {
		for _, proto := range server.Protocols {
			if !lo.Contains(s.Protocols, proto) {
				s.Protocols = append(s.Protocols, proto)
			}
		}
	}
	// Defaults to 10 to allow servers to be set lower for lower priority
	if s.Weight == 0 {
		s.Weight = 10
	}
	ips, err := net.LookupIP(u.Host)
	if err != nil {
		log.WithFields(log.Fields{
			"error":  err,
			"server": s.Host,
		}).Warning("Could not resolve address")
		return nil, err
	}

	// Check for IPv6 support using resolved IPs
	hasIPv6 := false
	for _, ip := range ips {
		if ip.To4() == nil && ip.To16() != nil {
			hasIPv6 = true
			break
		}
	}
	s.IPv6 = hasIPv6

	var city db.City
	err = r.db.Lookup(ips[0], &city)
	if err != nil {
		log.WithFields(log.Fields{
			"error":  err,
			"server": s.Host,
			"ip":     ips[0],
		}).Warning("Could not geolocate address")
		return nil, err
	}
	s.Country = city.Country.IsoCode
	if s.Continent == "" {
		s.Continent = city.Continent.Code
	}
	if s.Latitude == 0 && s.Longitude == 0 {
		s.Latitude = city.Location.Latitude
		s.Longitude = city.Location.Longitude
	}
	return s, nil
}

func (r *Redirector) reloadMap() error {
	mapFile := r.config.MapFile
	if mapFile == "" {
		return nil
	}
	log.WithField("file", mapFile).Info("Loading download map")
	newMap, err := loadMapFile(mapFile, r.config.SpecialExtensions)
	if err != nil {
		return err
	}
	r.dlMap = newMap
	return nil
}
