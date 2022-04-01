package main

import (
	"github.com/jmcvetta/randutil"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"math"
	"net"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	checkClient = &http.Client{
		Timeout: 10 * time.Second,
	}
)

type Server struct {
	Available bool               `json:"available"`
	Host      string             `json:"host"`
	Path      string             `json:"path"`
	Latitude  float64            `json:"latitude"`
	Longitude float64            `json:"longitude"`
	Weight    int                `json:"weight"`
	Continent string             `json:"continent"`
	Redirects prometheus.Counter `json:"-"`
}

func (server *Server) checkStatus() {
	req, err := http.NewRequest(http.MethodGet, "https://"+server.Host+"/"+strings.TrimLeft(server.Path, "/"), nil)

	req.Header.Set("User-Agent", "ArmbianRouter/1.0 (Go "+runtime.Version()+")")

	if err != nil {
		// This should never happen.
		log.WithFields(log.Fields{
			"server": server.Host,
			"error":  err,
		}).Warning("Invalid request! This should not happen, please check config.")
		return
	}

	res, err := checkClient.Do(req)

	if err != nil {
		if server.Available {
			log.WithFields(log.Fields{
				"server": server.Host,
				"error":  err,
			}).Info("Server went offline")

			server.Available = false
		} else {
			log.WithFields(log.Fields{
				"server": server.Host,
				"error":  err,
			}).Debug("Server is still offline")
		}
		return
	}

	responseFields := log.Fields{
		"server":       server.Host,
		"responseCode": res.StatusCode,
	}

	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusMovedPermanently || res.StatusCode == http.StatusFound || res.StatusCode == http.StatusNotFound {
		if !server.Available {
			server.Available = true
			log.WithFields(responseFields).Info("Server is online")
		}
	} else {
		log.WithFields(responseFields).Debug("Server status not known")

		if server.Available {
			log.WithFields(responseFields).Info("Server went offline")
			server.Available = false
		}
	}
}

type ServerList []*Server

func (s ServerList) checkLoop() {
	t := time.NewTicker(60 * time.Second)

	for {
		<-t.C
		s.Check()
	}
}

// Check will request the index from all servers
// If a server does not respond in 10 seconds, it is considered offline.
// This will wait until all checks are complete.
func (s ServerList) Check() {
	var wg sync.WaitGroup

	for _, server := range s {
		wg.Add(1)

		go func(server *Server) {
			defer wg.Done()

			server.checkStatus()
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

func (d DistanceList) Choices() []randutil.Choice {
	c := make([]randutil.Choice, len(d))

	for i, item := range d {
		c[i] = randutil.Choice{
			Weight: item.Server.Weight,
			Item:   item,
		}
	}

	return c
}

// Closest will use GeoIP on the IP provided and find the closest servers.
// When we have a list of x servers closest, we can choose a random or weighted one.
// Return values are the closest server, the distance, and if an error occurred.
func (s ServerList) Closest(ip net.IP) (*Server, float64, error) {
	choiceInterface, exists := serverCache.Get(ip.String())

	if !exists {
		var city LocationLookup
		err := db.Lookup(ip, &city)

		if err != nil {
			return nil, -1, err
		}

		c := make(DistanceList, len(s))

		for i, server := range s {
			if !server.Available {
				continue
			}

			c[i] = ComputedDistance{
				Server:   server,
				Distance: Distance(city.Location.Latitude, city.Location.Longitude, server.Latitude, server.Longitude),
			}
		}

		// Sort by distance
		sort.Slice(s, func(i int, j int) bool {
			return c[i].Distance < c[j].Distance
		})

		choiceInterface = c[0:topChoices].Choices()

		serverCache.Add(ip.String(), choiceInterface)
	}

	choice, err := randutil.WeightedChoice(choiceInterface.([]randutil.Choice))

	if err != nil {
		return nil, -1, err
	}

	dist := choice.Item.(ComputedDistance)

	return dist.Server, dist.Distance, nil
}

// haversin(θ) function
func hsin(theta float64) float64 {
	return math.Pow(math.Sin(theta/2), 2)
}

// Distance function returns the distance (in meters) between two points of
//     a given longitude and latitude relatively accurately (using a spherical
//     approximation of the Earth) through the Haversin Distance Formula for
//     great arc distance on a sphere with accuracy for small distances
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
