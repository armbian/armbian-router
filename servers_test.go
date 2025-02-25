package redirector

import (
	"encoding/json"
	"github.com/armbian/redirector/geo"
	lru "github.com/hashicorp/golang-lru"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"net"
)

// server1.example.com -> San Francisco, CA, USA (37.8749, -122.3194)
// server2.example.ca -> Ottawa, ON, Canada (45.5215, -75.5972)
// server3.example.com -> New York, NY, USA (40.8128, -73.906)
// server4.example.ca -> Vancouver, BC, Canada (49.3827, -123.0207)
// server5.example.com -> Los Angeles, CA, USA (34.1522, -118.1437)
// server6.example.ca -> Calgary, AB, Canada (51.1447, -114.1719)
// server7.example.com -> Chicago, IL, USA (41.9781, -87.5298)
// server8.example.ca -> Edmonton, AB, Canada (53.6461, -113.3938)
// server9.example.com -> Houston, TX, USA (29.8604, -95.2698)
// server10.example.ca -> Toronto, ON, Canada (43.7532, -79.2832)
// server11.example.com -> Chicago, IL, USA (42.1781, -87.7298) ~20km variation
// server12.example.com -> Chicago, IL, USA (42.5781, -87.9298) ~50km variation
// server13.example.com -> Detroit, MI, USA (42.3314, -83.0458)
// server14.example.com -> Detroit, MI, USA (42.5314, -83.2458) ~20km variation
const serverTestJson = `[
  {
    "available": true,
    "host": "server1.example.com",
    "latitude": 37.7749,
    "longitude": -122.4194,
	"weight": 10,
    "protocols": ["http", "https"]
  },
  {
    "available": true,
    "host": "server2.example.ca",
    "latitude": 45.4215,
    "longitude": -75.6972,
	"weight": 10,
    "protocols": ["http", "https"]
  },
  {
    "available": true,
    "host": "server3.example.com",
    "latitude": 40.7128,
    "longitude": -74.006,
	"weight": 10,
    "protocols": ["http", "https"]
  },
  {
    "available": true,
    "host": "server4.example.ca",
    "latitude": 49.2827,
    "longitude": -123.1207,
	"weight": 10,
    "protocols": ["http", "https"]
  },
  {
    "available": true,
    "host": "server5.example.com",
    "latitude": 34.0522,
    "longitude": -118.2437,
	"weight": 10,
    "protocols": ["http", "https"]
  },
  {
    "available": true,
    "host": "server6.example.ca",
    "latitude": 51.0447,
    "longitude": -114.0719,
	"weight": 10,
    "protocols": ["http", "https"]
  },
  {
    "available": true,
    "host": "server7.example.com",
    "latitude": 41.8781,
    "longitude": -87.6298,
	"weight": 10,
    "protocols": ["http", "https"]
  },
  {
    "available": true,
    "host": "server8.example.ca",
    "latitude": 53.5461,
    "longitude": -113.4938,
	"weight": 10,
    "protocols": ["http", "https"]
  },
  {
    "available": true,
    "host": "server9.example.com",
    "latitude": 29.7604,
    "longitude": -95.3698,
	"weight": 10,
    "protocols": ["http", "https"]
  },
  {
    "available": true,
    "host": "server10.example.ca",
    "latitude": 43.6532,
    "longitude": -79.3832,
	"weight": 10,
    "protocols": ["http", "https"]
  },
  {
    "available": true,
    "host": "server11.example.com",
    "latitude": 42.1781,
    "longitude": -87.7298,
    "protocols": ["http", "https"]
  },
  {
    "available": true,
    "host": "server12.example.com",
    "latitude": 42.5781,
    "longitude": -87.9298,
    "protocols": ["http", "https"]
  },
  {
    "available": true,
    "host": "server13.example.com",
    "latitude": 42.3314,
    "longitude": -83.0458,
    "protocols": ["http", "https"]
  },
  {
    "available": true,
    "host": "server14.example.com",
    "latitude": 42.5314,
    "longitude": -83.2458,
    "protocols": ["http", "https"]
  }
]
`

var _ = Describe("Servers", func() {
	var (
		r            *Redirector
		mockProvider *geo.MockProvider
	)

	BeforeEach(func() {
		log.SetLevel(log.DebugLevel)
		mockProvider = &geo.MockProvider{}
		r = New(&Config{})
		r.geo = mockProvider
		r.serverCache, _ = lru.New(10)

		err := json.Unmarshal([]byte(serverTestJson), &r.servers)

		Expect(err).To(BeNil())
	})

	Context("Single server", func() {
		BeforeEach(func() {
			// Single server returns
			r.config.TopChoices = 1
		})
		It("Should successfully return the closest server to San Francisco, CA", func() {
			ip := net.IPv4(1, 2, 3, 4)

			// This geolocations to San Francisco, CA, USA
			mockProvider.On("City", ip).Return(&geo.City{
				Location: geo.Location{Latitude: 37.8749, Longitude: -122.3194},
			}, nil)
			mockProvider.On("ASN", ip).Return(nil, geo.ErrNoASN)

			closest, distance, err := r.servers.Closest(r, "https", ip)

			Expect(err).To(BeNil())

			// Expect the closest server to be server1.example.com with a distance of around 14185
			Expect(closest.Host).To(Equal("server1.example.com"))

			// Distance is calculated beforehand and should be the same no matter how many runs due to presets
			Expect(int(distance)).To(Equal(14185))
		})
	})
	Context("Round-Robin Balancing", func() {
		BeforeEach(func() {
			// Multi server returns
			r.config.TopChoices = 5
			r.config.MaxDeviation = 50000 // 50km
		})
		It("Should successfully return a server in the closest area, when other servers are too far", func() {
			ip := net.IPv4(4, 3, 2, 1)

			// Ottawa, CA
			mockProvider.On("City", ip).Return(&geo.City{
				Location: geo.Location{Latitude: 45.5215, Longitude: -75.5972},
			}, nil)
			mockProvider.On("ASN", ip).Return(nil, geo.ErrNoASN)

			closest, distance, err := r.servers.Closest(r, "https", ip)

			Expect(err).To(BeNil())

			// This should ONLY return our Ottawa server due to the deviation setting
			Expect(closest.Host).To(Equal("server2.example.ca"))

			// This is the distance we expect
			Expect(int(distance)).To(Equal(13596))
		})
		It("Should successfully return a close server and not one from outside our deviation range", func() {
			ip := net.IPv4(4, 3, 2, 1)

			// Ottawa, CA
			mockProvider.On("City", ip).Return(&geo.City{
				// Near Ann Arbor, MI - outside of Detroit
				Location: geo.Location{Latitude: 42.2819, Longitude: -83.7538},
			}, nil)
			mockProvider.On("ASN", ip).Return(nil, geo.ErrNoASN)

			choices, err := r.servers.Choices(r, "https", ip)

			Expect(err).To(BeNil())

			// Expect only the Detroit servers
			Expect(len(choices)).To(Equal(2))

			for _, choice := range choices {
				item := choice.Item.(ComputedDistance)

				Expect(item.Server.Host).To(BeElementOf("server13.example.com", "server14.example.com"))
				Expect(item.Distance).To(BeNumerically("<", 60000))
			}
		})
		It("Should successfully return the top servers if they are all within reasonable distance", func() {

			ip := net.IPv4(4, 3, 2, 1)

			// Ottawa, CA
			mockProvider.On("City", ip).Return(&geo.City{
				// Grand Rapids, MI - in between Detroit and Chicago
				Location: geo.Location{Latitude: 42.9657, Longitude: -85.6774},
			}, nil)
			mockProvider.On("ASN", ip).Return(nil, geo.ErrNoASN)

			choices, err := r.servers.Choices(r, "https", ip)

			Expect(err).To(BeNil())

			Expect(len(choices)).To(Equal(r.config.TopChoices))

			for _, choice := range choices {
				item := choice.Item.(ComputedDistance)

				Expect(item.Server.Host).To(BeElementOf(
					"server7.example.com",  // Chicago, IL, USA
					"server11.example.com", // Chicago, IL, USA
					"server12.example.com", // Chicago, IL, USA
					"server13.example.com", // Detroit, MI
					"server14.example.com", // Detroit, MI
				))
			}
		})
	})
})
