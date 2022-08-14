package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"time"
)

var (
	ErrHttpsRedirect = errors.New("unexpected forced https redirect")
	ErrCertExpired   = errors.New("certificate is expired")
)

// checkHttp checks a URL for validity, and checks redirects
func checkHttp(server *Server, logFields log.Fields) (bool, error) {
	u := &url.URL{
		Scheme: "http",
		Host:   server.Host,
		Path:   server.Path,
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)

	req.Header.Set("User-Agent", "ArmbianRouter/1.0 (Go "+runtime.Version()+")")

	if err != nil {
		return false, err
	}

	res, err := checkClient.Do(req)

	if err != nil {
		return false, err
	}

	logFields["responseCode"] = res.StatusCode

	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusMovedPermanently || res.StatusCode == http.StatusFound || res.StatusCode == http.StatusNotFound {
		if res.StatusCode == http.StatusMovedPermanently || res.StatusCode == http.StatusFound {
			location := res.Header.Get("Location")

			logFields["url"] = location

			// Check that we don't redirect to https from a http url
			if u.Scheme == "http" {
				res, err := checkRedirect(location)

				if !res || err != nil {
					return res, err
				}
			}
		}

		return true, nil
	}

	logFields["cause"] = fmt.Sprintf("Unexpected http status %d", res.StatusCode)

	return false, nil
}

// checkRedirect parses a location header response and checks the scheme
func checkRedirect(locationHeader string) (bool, error) {
	newUrl, err := url.Parse(locationHeader)

	if err != nil {
		return false, err
	}

	if newUrl.Scheme == "https" {
		return false, ErrHttpsRedirect
	}

	return true, nil
}

// checkTLS checks tls certificates from a host, ensures they're valid, and not expired.
func checkTLS(server *Server, logFields log.Fields) (bool, error) {
	host, port, err := net.SplitHostPort(server.Host)

	if port == "" {
		port = "443"
	}

	conn, err := tls.Dial("tcp", host+":"+port, checkTLSConfig)

	if err != nil {
		return false, err
	}

	defer conn.Close()

	err = conn.VerifyHostname(server.Host)

	if err != nil {
		return false, err
	}

	now := time.Now()

	state := conn.ConnectionState()

	opts := x509.VerifyOptions{
		CurrentTime: time.Now(),
	}

	for _, cert := range state.PeerCertificates {
		if _, err := cert.Verify(opts); err != nil {
			logFields["peerCert"] = cert.Subject.String()
			return false, err
		}
		if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
			return false, err
		}
	}

	for _, chain := range state.VerifiedChains {
		for _, cert := range chain {
			if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
				logFields["cert"] = cert.Subject.String()
				return false, ErrCertExpired
			}
		}
	}

	return true, nil
}
