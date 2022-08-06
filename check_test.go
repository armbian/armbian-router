package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

var _ = Describe("Check suite", func() {
	Context("HTTP Checks", func() {
		var (
			httpServer *httptest.Server
			server     *Server
			handler    http.HandlerFunc
		)
		BeforeEach(func() {
			httpServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handler(w, r)
			}))

			u, err := url.Parse(httpServer.URL)

			if err != nil {
				panic(err)
			}

			server = &Server{
				Host: u.Host,
				Path: u.Path,
			}
		})
		AfterEach(func() {
			httpServer.Close()
		})
		It("Should successfully check for connectivity", func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}

			res, err := checkHttp(server, log.Fields{})

			Expect(res).To(BeTrue())
			Expect(err).To(BeNil())
		})
		It("Should return an error when redirected to https", func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", strings.Replace(httpServer.URL, "http://", "https://", -1))
				w.WriteHeader(http.StatusMovedPermanently)
			}

			res, err := checkHttp(server, log.Fields{})

			Expect(res).To(BeFalse())
			Expect(err).To(Equal(ErrHttpsRedirect))
		})
	})
	Context("TLS Checks", func() {

	})
})
