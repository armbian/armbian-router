package redirector

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"
)

func genTestCerts(notBefore, notAfter time.Time) (*pem.Block, *pem.Block, error) {
	// Create a Certificate Authority Cert
	template := x509.Certificate{
		SerialNumber:          big.NewInt(0),
		Subject:               pkix.Name{CommonName: "localhost"},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	// Create a Private Key
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not generate rsa key - %s", err)
	}

	// Use CA Cert to sign a CSR and create a Public Cert
	csr := &key.PublicKey
	cert, err := x509.CreateCertificate(rand.Reader, &template, &template, csr, key)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not generate certificate - %s", err)
	}

	// Convert keys into pem.Block
	c := &pem.Block{Type: "CERTIFICATE", Bytes: cert}
	k := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return c, k, nil
}

var _ = Describe("Check suite", func() {
	var (
		httpServer *httptest.Server
		server     *Server
		handler    http.HandlerFunc
		r          *Redirector
	)
	BeforeEach(func() {
		httpServer = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler(w, r)
		}))

		r = New(&Config{
			checkClient: &http.Client{},
		})

		r.config.SetRootCAs(x509.NewCertPool())

		Expect(r.config).ToNot(BeNil())
	})
	AfterEach(func() {
		httpServer.Close()
	})
	setupServer := func() {
		u, err := url.Parse(httpServer.URL)

		if err != nil {
			panic(err)
		}

		server = &Server{
			Host: u.Host,
			Path: u.Path,
			Protocols: []string{
				"http",
			},
		}
	}

	Context("HTTP Checks", func() {
		var h *HTTPCheck
		BeforeEach(func() {
			httpServer.Start()
			setupServer()
			h = &HTTPCheck{config: r.config}
		})
		It("Should successfully check for connectivity", func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}

			res, err := h.checkHTTPScheme(server, "http", log.Fields{})

			Expect(res).To(BeTrue())
			Expect(err).To(BeNil())
		})
	})
	Context("TLS Checks", func() {
		var (
			x509Cert *x509.Certificate
			t        *TLSCheck
		)
		setupCerts := func(notBefore, notAfter time.Time) {
			cert, key, err := genTestCerts(notBefore, notAfter)

			if err != nil {
				panic("Unable to generate test certs")
			}

			x509Cert, err = x509.ParseCertificate(cert.Bytes)

			if err != nil {
				panic("Unable to parse certificate from bytes: " + err.Error())
			}

			tlsPair, err := tls.X509KeyPair(pem.EncodeToMemory(cert), pem.EncodeToMemory(key))

			if err != nil {
				panic("Unable to load tls key pair: " + err.Error())
			}

			httpServer.TLS = &tls.Config{
				Certificates: []tls.Certificate{tlsPair},
			}

			pool := x509.NewCertPool()

			pool.AddCert(x509Cert)

			r.config.SetRootCAs(pool)

			t = &TLSCheck{config: r.config}

			httpServer.StartTLS()
			setupServer()
		}
		Context("HTTPS Checks", func() {
			var h *HTTPCheck
			BeforeEach(func() {
				h = &HTTPCheck{
					config: r.config,
				}
				setupServer()

				setupCerts(time.Now(), time.Now().Add(24*time.Hour))
			})
			It("Should return an error when redirected to http from https", func() {
				handler = func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Location", strings.Replace(httpServer.URL, "https://", "http://", -1))
					w.WriteHeader(http.StatusMovedPermanently)
				}

				logFields := log.Fields{}

				res, err := h.checkHTTPScheme(server, "https", logFields)

				Expect(logFields["url"]).ToNot(BeEmpty())
				Expect(logFields["url"]).ToNot(Equal(httpServer.URL))
				Expect(err).To(Equal(ErrHTTPRedirect))
				Expect(res).To(BeFalse())
			})
		})
		Context("CA Tests", func() {
			BeforeEach(func() {
				setupServer()
				setupCerts(time.Now(), time.Now().Add(24*time.Hour))
			})
			It("Should fail due to invalid ca", func() {
				r.config.SetRootCAs(x509.NewCertPool())

				res, err := t.Check(server, log.Fields{})

				Expect(res).To(BeFalse())
				Expect(err).ToNot(BeNil())
			})
			It("Should successfully validate certificates (valid ca, valid date/times, etc)", func() {
				res, err := t.Check(server, log.Fields{})

				Expect(res).To(BeFalse())
				Expect(err).ToNot(BeNil())
			})
		})
		Context("Expiration tests", func() {
			It("Should fail due to not yet valid certificate", func() {
				setupCerts(time.Now().Add(5*time.Hour), time.Now().Add(10*time.Hour))

				// Check TLS
				res, err := t.Check(server, log.Fields{})

				Expect(res).To(BeFalse())
				Expect(err).ToNot(BeNil())
			})
			It("Should fail due to expired certificate", func() {
				setupCerts(time.Now().Add(-10*time.Hour), time.Now().Add(-5*time.Hour))

				// Check TLS
				res, err := t.Check(server, log.Fields{})

				Expect(res).To(BeFalse())
				Expect(err).ToNot(BeNil())
			})
		})
		Context("Version checks", func() {
			var v *VersionCheck
			BeforeEach(func() {
				v = &VersionCheck{
					config:          r.config,
					lastVersion:     "1234567890",
					lastVersionTime: time.Now(),
				}
				httpServer.Start()
				setupServer()
			})
			It("Should succeed and match versions", func() {
				handler = func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("1234567890"))
				}

				res, err := v.Check(server, log.Fields{})

				Expect(err).To(BeNil())
				Expect(res).To(BeTrue())
			})
			It("Should fail due to mismatched versions", func() {
				handler = func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("0987654321"))
				}

				res, err := v.Check(server, log.Fields{})

				Expect(res).To(BeFalse())
				Expect(err).ToNot(BeNil())
			})
		})
	})
})
