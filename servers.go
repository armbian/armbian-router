package redirector

import (
	"fmt"
	"github.com/armbian/redirector/db"
	"github.com/armbian/redirector/util"
	"github.com/jmcvetta/randutil"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
	"math"
	"net"
	"reflect"
	"sort"
	"sync"
	"time"
)

// Server represents a download server
type Server struct {
	Available  bool               `json:"available"`
	Reason     string             `json:"reason,omitempty"`
	Host       string             `json:"host"`
	Path       string             `json:"path"`
	Latitude   float64            `json:"latitude"`
	Longitude  float64            `json:"longitude"`
	Weight     int                `json:"weight"`
	Continent  string             `json:"continent"`
	Country    string             `json:"country"`
	Protocols  []string           `json:"protocols"`
	Rules      []Rule             `json:"rules,omitempty"`
	Redirects  prometheus.Counter `json:"-"`
	LastChange time.Time          `json:"lastChange"`
}

// ServerCheck is a check function which can return information about a status.
type ServerCheck interface {
	Check(server *Server, logFields log.Fields) (bool, error)
}

// checkStatus runs all status checks against a server
// The return value of this isn't the availability, rather the change status
// If true, the cache is flushed. If false, it does nothing.
func (s *Server) checkStatus(checks []ServerCheck) bool {
	logFields := log.Fields{
		"host": s.Host,
	}

	var res bool
	var err error

	for _, check := range checks {
		res, err = check.Check(s, logFields)

		if err != nil {
			logFields["error"] = err
		}

		if !res {
			checkType := reflect.TypeOf(check)

			if checkType.Kind() == reflect.Ptr {
				checkType = checkType.Elem()
			}

			logFields["check"] = checkType.Name()
			break
		}
	}

	if !res {
		if s.Available {
			log.WithFields(logFields).Info("Server is now unavailable")

			s.Available = false
			s.LastChange = time.Now()

			if v, ok := logFields["error"]; ok {
				s.Reason = fmt.Sprintf("%v", v)
			}

			return true
		} else {
			log.WithFields(logFields).Debug("Server is still unavailable")
		}

		return false
	}

	if !s.Available {
		s.Available = true
		s.Reason = ""
		s.LastChange = time.Now()

		log.WithFields(logFields).Info("Server is online")

		return true
	}

	return false
}

// checkRUles takes input from a value match and checks the ruleset.
// This will remove items for ASN rules, etc.
func (s *Server) checkRules(input RuleInput) bool {
	if len(s.Rules) < 1 {
		return true
	}

	for _, rule := range s.Rules {
		value, ok := util.GetValue(input, rule.Field)

		if !ok {
			log.WithFields(log.Fields{
				"field": rule.Field,
			}).Warning("Invalid rule field")
			continue
		}

		valueStr := fmt.Sprintf("%v", value)

		if len(rule.Is) > 0 && rule.Is != valueStr {
			return false
		} else if len(rule.IsNot) > 0 && rule.IsNot == valueStr {
			return false
		} else if len(rule.In) > 0 && !lo.Contains(rule.In, valueStr) {
			return false
		} else if len(rule.NotIn) > 0 && lo.Contains(rule.NotIn, valueStr) {
			return false
		}
	}

	return true
}

// ServerList is a wrapper for a Server slice.
// It implements features like server checks.
type ServerList []*Server

// checkLoop is a loop function which checks server statuses
// every 60 seconds.
func (s ServerList) checkLoop(r *Redirector, checks []ServerCheck) {
	t := time.NewTicker(60 * time.Second)

	for {
		<-t.C
		s.Check(r, checks)
	}
}

// Check will request the index from all servers
// If a server does not respond in 10 seconds, it is considered offline.
// This will wait until all checks are complete.
func (s ServerList) Check(r *Redirector, checks []ServerCheck) {
	p := pool.New()

	var clearOnce sync.Once

	f := func(server *Server) func() {
		return func() {
			if !server.checkStatus(checks) {
				return
			}

			// Clear cache, but only once
			clearOnce.Do(func() {
				r.serverCache.Purge()
			})
		}
	}

	for _, server := range s {
		p.Go(f(server))
	}

	p.Wait()
}

// RuleInput is a set of fields used for rule checks
type RuleInput struct {
	IP       string  `json:"ip"`
	ASN      db.ASN  `json:"asn"`
	Location db.City `json:"location"`
}

// ComputedDistance is a wrapper that contains a Server and Distance.
type ComputedDistance struct {
	Server   *Server
	Distance float64
}

// Closest uses GeoIP on the client's IP and compares the client's location
// with that of the servers. If there are servers with the same country code,
// it computes the distances. If the nearest server is within a threshold (e.g. 50km),
// it is selected deterministically; otherwise, a weighted selection is used.
// If no local servers exist, it falls back to a weighted selection among all valid servers.
func (s ServerList) Closest(r *Redirector, scheme string, ip net.IP) (*Server, float64, error) {
    cacheKey := scheme + "_" + ip.String()

    if cached, exists := r.serverCache.Get(cacheKey); exists {
        if comp, ok := cached.(ComputedDistance); ok {
            log.Infof("Cache hit: %s", comp.Server.Host)
            return comp.Server, comp.Distance, nil
        }
        r.serverCache.Remove(cacheKey)
    }

    var city db.City
    if err := r.db.Lookup(ip, &city); err != nil {
        log.WithError(err).Warning("Unable to lookup client location")
        return nil, -1, err
    }
    clientCountry := city.Country.IsoCode

    var asn db.ASN
    if r.asnDB != nil {
        if err := r.asnDB.Lookup(ip, &asn); err != nil {
            log.WithError(err).Warning("Unable to load ASN information")
            return nil, -1, err
        }
    }

    ruleInput := RuleInput{
        IP:       ip.String(),
        ASN:      asn,
        Location: city,
    }

    validServers := lo.Filter(s, func(server *Server, _ int) bool {
        if !server.Available || !lo.Contains(server.Protocols, scheme) {
            return false
        }
        if len(server.Rules) > 0 && !server.checkRules(ruleInput) {
            log.WithField("host", server.Host).Debug("Skipping server due to rules")
            return false
        }
        return true
    })

    if len(validServers) < 2 {
        validServers = s
    }

    localServers := lo.Filter(validServers, func(server *Server, _ int) bool {
        return server.Country == clientCountry
    })

    if len(localServers) > 0 {
        computedLocal := lo.Map(localServers, func(server *Server, _ int) ComputedDistance {
            d := Distance(city.Location.Latitude, city.Location.Longitude, server.Latitude, server.Longitude)
            return ComputedDistance{
                Server:   server,
                Distance: d,
            }
        })

        sort.Slice(computedLocal, func(i, j int) bool {
            return computedLocal[i].Distance < computedLocal[j].Distance
        })

        if computedLocal[0].Distance < r.config.SameCityThreshold {
            chosen := computedLocal[0]
            r.serverCache.Add(cacheKey, chosen)
            return chosen.Server, chosen.Distance, nil
        }

        choiceCount := r.config.TopChoices
        if len(computedLocal) < choiceCount {
            choiceCount = len(computedLocal)
        }

        choices := make([]randutil.Choice, choiceCount)
        for i, item := range computedLocal[:choiceCount] {
            choices[i] = randutil.Choice{
                Weight: item.Server.Weight,
                Item:   item,
            }
        }

        choice, err := randutil.WeightedChoice(choices)
        if err != nil {
            log.WithError(err).Warning("Unable to choose a weighted choice")
            return nil, -1, err
        }

        dist := choice.Item.(ComputedDistance)
        r.serverCache.Add(cacheKey, dist)
        return dist.Server, dist.Distance, nil
    }

	// Fallback: if no local servers exist, simply select the nearest server among all valid servers.
    computed := lo.Map(validServers, func(server *Server, _ int) ComputedDistance {
        d := Distance(city.Location.Latitude, city.Location.Longitude, server.Latitude, server.Longitude)
        return ComputedDistance{
            Server:   server,
            Distance: d,
        }
    })

    sort.Slice(computed, func(i, j int) bool {
        return computed[i].Distance < computed[j].Distance
    })

    choiceCount := r.config.TopChoices
    if len(computed) < choiceCount {
        choiceCount = len(computed)
    }

    choices := make([]randutil.Choice, choiceCount)
    for i, item := range computed[:choiceCount] {
        choices[i] = randutil.Choice{
            Weight: item.Server.Weight,
            Item:   item,
        }
    }

    choice, err := randutil.WeightedChoice(choices)
    if err != nil {
        log.WithError(err).Warning("Unable to choose a weighted choice")
        return nil, -1, err
    }

    dist := choice.Item.(ComputedDistance)
    r.serverCache.Add(cacheKey, dist)
    return dist.Server, dist.Distance, nil
}

// haversin(θ) function
func hsin(theta float64) float64 {
	return math.Pow(math.Sin(theta/2), 2)
}

// Distance function returns the distance (in meters) between two points of
//
//	a given longitude and latitude relatively accurately (using a spherical
//	approximation of the Earth) through the Haversine Distance Formula for
//	great arc distance on a sphere with accuracy for small distances
//
// point coordinates are supplied in degrees and converted into rad. in the func
//
// distance returned is METERS!!!!!!
// http://en.wikipedia.org/wiki/Haversine_formula
func Distance(lat1, lon1, lat2, lon2 float64) float64 {
	// convert to radians
	// must cast radius as float to multiply later
	var la1, lo1, la2, lo2, r float64
	la1 = lat1 * math.Pi / 180
	lo1 = lon1 * math.Pi / 180
	la2 = lat2 * math.Pi / 180
	lo2 = lon2 * math.Pi / 180

	r = 6378100 // Earth radius in METERS

	// calculate
	h := hsin(la2-la1) + math.Cos(la1)*math.Cos(la2)*hsin(lo2-lo1)

	return 2 * r * math.Asin(math.Sqrt(h))
}
