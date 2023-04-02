package redirector

import (
	"fmt"
	"github.com/jmcvetta/randutil"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
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
func (s *Server) checkStatus(checks []ServerCheck) {
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
		} else {
			log.WithFields(logFields).Debug("Server is still unavailable")
		}

		return
	}

	if !s.Available {
		s.Available = true
		s.Reason = ""
		s.LastChange = time.Now()

		log.WithFields(logFields).Info("Server is online")
	}
}

// checkRUles takes input from a value match and checks the ruleset.
// This will remove items for ASN rules, etc.
func (s *Server) checkRules(input RuleInput) bool {
	if len(s.Rules) < 1 {
		return true
	}

	for _, rule := range s.Rules {
		value, ok := GetValue(input, rule.Field)

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
func (s ServerList) checkLoop(checks []ServerCheck) {
	t := time.NewTicker(60 * time.Second)

	for {
		<-t.C
		s.Check(checks)
	}
}

// Check will request the index from all servers
// If a server does not respond in 10 seconds, it is considered offline.
// This will wait until all checks are complete.
func (s ServerList) Check(checks []ServerCheck) {
	var wg sync.WaitGroup

	for _, server := range s {
		wg.Add(1)

		go func(server *Server) {
			defer wg.Done()

			server.checkStatus(checks)
		}(server)
	}

	wg.Wait()
}

type RuleInput struct {
	IP       string `json:"ip"`
	ASN      ASN    `json:"asn"`
	Location City   `json:"location"`
}

// ComputedDistance is a wrapper that contains a Server and Distance.
type ComputedDistance struct {
	Server   *Server
	Distance float64
}

// Closest will use GeoIP on the IP provided and find the closest servers.
// When we have a list of x servers closest, we can choose a random or weighted one.
// Return values are the closest server, the distance, and if an error occurred.
func (s ServerList) Closest(r *Redirector, scheme string, ip net.IP) (*Server, float64, error) {
	choiceInterface, exists := r.serverCache.Get(scheme + "_" + ip.String())

	if !exists {
		var city City
		err := r.db.Lookup(ip, &city)

		if err != nil {
			log.WithError(err).Warning("Unable to lookup location information")
			return nil, -1, err
		}

		var asn ASN

		if r.asnDB != nil {
			err = r.asnDB.Lookup(ip, &asn)

			if err != nil {
				log.WithError(err).Warning("Unable to load ASN information")
				return nil, -1, err
			}
		}

		ruleInput := RuleInput{
			IP:       ip.String(),
			ASN:      asn,
			Location: city,
		}

		// First, filter our servers to what are actually available/match.
		validServers := lo.Filter(s, func(server *Server, _ int) bool {
			if !server.Available || !lo.Contains(server.Protocols, scheme) {
				return false
			}

			if !server.checkRules(ruleInput) {
				log.WithField("host", server.Host).Debug("Skipping server due to rules")
				return false
			}

			return true
		})

		// Then, map them to distances from the client
		c := lo.Map(validServers, func(server *Server, _ int) ComputedDistance {
			distance := Distance(city.Location.Latitude, city.Location.Longitude, server.Latitude, server.Longitude)

			return ComputedDistance{
				Server:   server,
				Distance: distance,
			}
		})

		// Sort by distance
		sort.Slice(c, func(i int, j int) bool {
			return c[i].Distance < c[j].Distance
		})

		choiceCount := r.config.TopChoices

		if len(c) < r.config.TopChoices {
			choiceCount = len(c)
		}

		log.WithFields(log.Fields{"count": len(c)}).Debug("Picking from top choices")

		choices := make([]randutil.Choice, choiceCount)

		for i, item := range c[0:choiceCount] {
			choices[i] = randutil.Choice{
				Weight: item.Server.Weight,
				Item:   item,
			}
		}

		choiceInterface = choices

		r.serverCache.Add(scheme+"_"+ip.String(), choiceInterface)
	}

	choice, err := randutil.WeightedChoice(choiceInterface.([]randutil.Choice))

	if err != nil {
		log.WithError(err).Warning("Unable to choose a weighted choice")
		return nil, -1, err
	}

	dist := choice.Item.(ComputedDistance)

	if !dist.Server.Available {
		// Choose a new server and refresh cache
		r.serverCache.Remove(scheme + "_" + ip.String())

		return s.Closest(r, scheme, ip)
	}

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
