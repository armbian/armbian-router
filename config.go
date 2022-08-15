package redirector

import (
	"crypto/x509"
	lru "github.com/hashicorp/golang-lru"
	"github.com/oschwald/maxminddb-golang"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
	"net"
	"net/url"
	"strings"
	"sync"
)

type Config struct {
	BindAddress string         `mapstructure:"bind"`
	GeoDBPath   string         `mapstructure:"geodb"`
	MapFile     string         `mapstructure:"dl_map"`
	CacheSize   int            `mapstructure:"cacheSize"`
	TopChoices  int            `mapstructure:"topChoices"`
	ReloadToken string         `mapstructure:"reloadToken"`
	ServerList  []ServerConfig `mapstructure:"servers"`
	ReloadFunc  func()
	RootCAs     *x509.CertPool
}

type ProtocolList []string

func (p ProtocolList) Contains(value string) bool {
	for _, val := range p {
		if value == val {
			return true
		}
	}

	return false
}

func (p ProtocolList) Append(value string) ProtocolList {
	return append(p, value)
}

func (p ProtocolList) Remove(value string) ProtocolList {
	index := -1

	for i, val := range p {
		if value == val {
			index = i
			break
		}
	}

	if index == -1 {
		return p
	}

	p[index] = p[len(p)-1]
	return p[:len(p)-1]
}

func (r *Redirector) ReloadConfig() error {
	log.Info("Loading configuration...")

	var err error

	// Load maxmind database
	if r.db != nil {
		err = r.db.Close()

		if err != nil {
			return errors.Wrap(err, "Unable to close database")
		}
	}

	// db can be hot-reloaded if the file changed
	r.db, err = maxminddb.Open(r.config.GeoDBPath)

	if err != nil {
		return errors.Wrap(err, "Unable to open database")
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
	if r.config.TopChoices > len(r.servers) {
		r.config.TopChoices = len(r.servers)
	}

	// Force check
	go r.servers.Check(r.checks)

	return nil
}

func (r *Redirector) reloadServers() error {
	log.WithField("count", len(r.config.ServerList)).Info("Loading servers")
	var wg sync.WaitGroup

	existing := make(map[string]int)

	for i, server := range r.servers {
		existing[server.Host] = i
	}

	hosts := make(map[string]bool)

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

		hosts[u.Host] = true

		i := -1

		if v, exists := existing[u.Host]; exists {
			i = v
		}

		go func(i int, server ServerConfig, u *url.URL) {
			defer wg.Done()

			s := r.addServer(server, u)

			if _, ok := existing[u.Host]; ok {
				s.Redirects = r.servers[i].Redirects

				r.servers[i] = s
			} else {
				s.Redirects = promauto.NewCounter(prometheus.CounterOpts{
					Name: "armbian_router_redirects_" + metricReplacer.Replace(u.Host),
					Help: "The number of redirects for server " + u.Host,
				})

				r.servers = append(r.servers, s)

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
	for i := len(r.servers) - 1; i >= 0; i-- {
		if _, exists := hosts[r.servers[i].Host]; exists {
			continue
		}

		log.WithFields(log.Fields{
			"server": r.servers[i].Host,
		}).Info("Removed server")

		r.servers = append(r.servers[:i], r.servers[i+1:]...)
	}

	return nil
}

var metricReplacer = strings.NewReplacer(".", "_", "-", "_")

// addServer takes ServerConfig and constructs a server.
// This will create duplicate servers, but it will overwrite existing ones when changed.
func (r *Redirector) addServer(server ServerConfig, u *url.URL) *Server {
	s := &Server{
		Available: true,
		Host:      u.Host,
		Path:      u.Path,
		Latitude:  server.Latitude,
		Longitude: server.Longitude,
		Continent: server.Continent,
		Weight:    server.Weight,
		Protocols: ProtocolList{"http", "https"},
	}

	if len(server.Protocols) > 0 {
		for _, proto := range server.Protocols {
			if !s.Protocols.Contains(proto) {
				s.Protocols = s.Protocols.Append(proto)
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
		return nil
	}

	var city City
	err = r.db.Lookup(ips[0], &city)

	if err != nil {
		log.WithFields(log.Fields{
			"error":  err,
			"server": s.Host,
			"ip":     ips[0],
		}).Warning("Could not geolocate address")
		return nil
	}

	if s.Continent == "" {
		s.Continent = city.Continent.Code
	}

	if s.Latitude == 0 && s.Longitude == 0 {
		s.Latitude = city.Location.Latitude
		s.Longitude = city.Location.Longitude
	}

	return s
}

func (r *Redirector) reloadMap() error {
	mapFile := r.config.MapFile

	if mapFile == "" {
		return nil
	}

	log.WithField("file", mapFile).Info("Loading download map")

	newMap, err := loadMapFile(mapFile)

	if err != nil {
		return err
	}

	r.dlMap = newMap

	return nil
}
