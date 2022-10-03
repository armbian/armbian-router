package redirector

import (
	"github.com/jmcvetta/randutil"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"math"
	"net"
	"sort"
	"sync"
	"time"
)

// Server represents a download server
type Server struct {
	Available  bool               `json:"available"`
	Host       string             `json:"host"`
	Path       string             `json:"path"`
	Latitude   float64            `json:"latitude"`
	Longitude  float64            `json:"longitude"`
	Weight     int                `json:"weight"`
	Continent  string             `json:"continent"`
	Protocols  ProtocolList       `json:"protocols"`
	IncludeASN ASNList            `json:"includeASN"`
	ExcludeASN ASNList            `json:"excludeASN"`
	Redirects  prometheus.Counter `json:"-"`
	LastChange time.Time          `json:"lastChange"`
}

type ServerCheck func(server *Server, logFields log.Fields) (bool, error)

// checkStatus runs all status checks against a server
func (server *Server) checkStatus(checks []ServerCheck) {
	logFields := log.Fields{
		"host": server.Host,
	}

	var res bool
	var err error

	for _, check := range checks {
		res, err = check(server, logFields)

		if err != nil {
			logFields["error"] = err
		}

		if !res {
			break
		}
	}

	if !res {
		if server.Available {
			log.WithFields(logFields).Info("Server went offline")

			server.Available = false
			server.LastChange = time.Now()
		} else {
			log.WithFields(logFields).Debug("Server is still offline")
		}

		return
	} else {
		if !server.Available {
			server.Available = true
			server.LastChange = time.Now()
			log.WithFields(logFields).Info("Server is online")
		}
	}
}

type ServerList []*Server

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

// ComputedDistance is a wrapper that contains a Server and Distance.
type ComputedDistance struct {
	Server   *Server
	Distance float64
}

// DistanceList is a list of Computed Distances with an easy "Choices" func
type DistanceList []ComputedDistance

// Closest will use GeoIP on the IP provided and find the closest servers.
// When we have a list of x servers closest, we can choose a random or weighted one.
// Return values are the closest server, the distance, and if an error occurred.
func (s ServerList) Closest(r *Redirector, scheme string, ip net.IP) (*Server, float64, error) {
	choiceInterface, exists := r.serverCache.Get(scheme + "_" + ip.String())

	if !exists {
		var city LocationLookup
		err := r.db.Lookup(ip, &city)

		if err != nil {
			return nil, -1, err
		}

		var asn ASN
		hasASN := false

		if r.asnDB != nil {
			err = r.asnDB.Lookup(ip, &asn)

			if err != nil {
				return nil, -1, err
			}

			hasASN = true
		}

		c := make(DistanceList, len(s))

		for i, server := range s {
			if !server.Available ||
				!server.Protocols.Contains(scheme) ||
				len(server.IncludeASN) > 0 && hasASN && !server.IncludeASN.Contains(asn.AutonomousSystemNumber) ||
				len(server.ExcludeASN) > 0 && hasASN && server.ExcludeASN.Contains(asn.AutonomousSystemNumber) {
				continue
			}

			distance := Distance(city.Location.Latitude, city.Location.Longitude, server.Latitude, server.Longitude)

			c[i] = ComputedDistance{
				Server:   server,
				Distance: distance,
			}
		}

		// Sort by distance
		sort.Slice(c, func(i int, j int) bool {
			return c[i].Distance < c[j].Distance
		})

		choiceCount := r.config.TopChoices

		if len(c) < r.config.TopChoices {
			choiceCount = len(c)
		}

		choices := make([]randutil.Choice, choiceCount)

		for i, item := range c[0:choiceCount] {
			if item.Server == nil {
				continue
			}

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
