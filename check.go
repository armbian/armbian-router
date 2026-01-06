package redirector

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
)

var (
	// ErrHTTPSRedirect is an error thrown when the webserver returns
	// an https redirect for an http url.
	ErrHTTPSRedirect = errors.New("unexpected forced https redirect")

	// ErrHTTPRedirect is an error thrown when the webserver returns
	// a redirect to a non-https url from an https request.
	ErrHTTPRedirect = errors.New("unexpected redirect to insecure url")

	// ErrCertExpired is a fatal error thrown when the webserver's
	// certificate is expired.
	ErrCertExpired = errors.New("certificate is expired")
)

// HTTPCheck is a check for validity and redirects
type HTTPCheck struct {
	config *Config
}

// Check checks a URL for validity, and checks redirects
func (h *HTTPCheck) Check(server *Server, logFields log.Fields) (bool, error) {
	return h.checkHTTPScheme(server, "http", logFields)
}

// checkHTTPScheme will check if a scheme is valid and doesn't redirect
func (h *HTTPCheck) checkHTTPScheme(server *Server, scheme string, logFields log.Fields) (bool, error) {
	u := &url.URL{
		Scheme: scheme,
		Host:   server.Host,
		Path:   server.Path,
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("User-Agent", "ArmbianRouter/1.0 (Go "+runtime.Version()+")")

	res, err := h.config.checkClient.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	logFields["responseCode"] = res.StatusCode

	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusMovedPermanently || res.StatusCode == http.StatusPermanentRedirect || res.StatusCode == http.StatusFound || res.StatusCode == http.StatusNotFound {
		if res.StatusCode == http.StatusMovedPermanently || res.StatusCode == http.StatusFound || res.StatusCode == http.StatusPermanentRedirect {
			location := res.Header.Get("Location")

			logFields["url"] = location

			switch u.Scheme {
			case "http":
				res, err := h.checkRedirect(u.Scheme, location)

				if !res || err != nil {
					// If we don't support http, we remove it from supported protocols
					server.mu.Lock()
					server.Protocols = Remove(server.Protocols, "http")
					server.mu.Unlock()
				} else {
					// Otherwise, we verify https support
					h.checkProtocol(server, "https")
				}
			case "https":
				// We don't want to allow downgrading, so this is an error.
				return h.checkRedirect(u.Scheme, location)
			}
		}

		return true, nil
	}

	logFields["cause"] = fmt.Sprintf("Unexpected http status %d", res.StatusCode)

	return false, nil
}

func (h *HTTPCheck) checkProtocol(server *Server, scheme string) {
	res, err := h.checkHTTPScheme(server, scheme, log.Fields{})

	if !res || err != nil {
		return
	}

	if !lo.Contains(server.Protocols, scheme) {
		server.mu.Lock()
		server.Protocols = append(server.Protocols, scheme)
		server.mu.Unlock()
	}
}

// checkRedirect parses a location header response and checks the scheme
func (h *HTTPCheck) checkRedirect(originatingScheme, locationHeader string) (bool, error) {
	newURL, err := url.Parse(locationHeader)

	if err != nil {
		return false, err
	}

	if newURL.Scheme == "https" {
		return false, ErrHTTPSRedirect
	} else if originatingScheme == "https" && newURL.Scheme == "http" {
		return false, ErrHTTPRedirect
	}

	return true, nil
}

// TLSCheck is a TLS certificate check
type TLSCheck struct {
	config *Config
}

// Check checks tls certificates from a host, ensures they're valid, and not expired.
func (t *TLSCheck) Check(server *Server, logFields log.Fields) (bool, error) {
	var host, port string
	var err error

	if strings.Contains(server.Host, ":") {
		host, port, err = net.SplitHostPort(server.Host)

		if err != nil {
			return false, err
		}
	} else {
		host = server.Host
	}

	log.WithFields(log.Fields{
		"server": server.Host,
		"host":   host,
		"port":   port,
	}).Debug("Checking TLS server")

	if port == "" {
		port = "443"
	}

	conn, err := tls.Dial("tcp", host+":"+port, &tls.Config{
		RootCAs: t.config.RootCAs,
	})

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

	peerPool := x509.NewCertPool()

	for _, intermediate := range state.PeerCertificates {
		if !intermediate.IsCA {
			continue
		}

		peerPool.AddCert(intermediate)
	}

	opts := x509.VerifyOptions{
		Roots:         t.config.RootCAs,
		Intermediates: peerPool,
		CurrentTime:   time.Now(),
	}

	// We want only the leaf certificate, as this will verify up the chain for us.
	cert := state.PeerCertificates[0]

	if _, err := cert.Verify(opts); err != nil {
		logFields["peerCert"] = cert.Subject.String()

		if authErr, ok := err.(x509.UnknownAuthorityError); ok {
			logFields["authCert"] = authErr.Cert.Subject.String()
			logFields["ca"] = authErr.Cert.Issuer
		}
		return false, err
	}

	if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
		logFields["peerCert"] = cert.Subject.String()
		return false, err
	}

	for _, chain := range state.VerifiedChains {
		for _, cert := range chain {
			if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
				logFields["cert"] = cert.Subject.String()
				return false, ErrCertExpired
			}
		}
	}

	// If https is valid, append it
	if !lo.Contains(server.Protocols, "https") {
		server.mu.Lock()
		server.Protocols = append(server.Protocols, "https")
		server.mu.Unlock()
	}

	return true, nil
}

// IPv6Check verifies that a server has a valid IPv6 address by checking for AAAA records.
type IPv6Check struct {
	config *Config
}

// Check verifies IPv6 support for the server by checking for AAAA records
func (i *IPv6Check) Check(server *Server, logFields log.Fields) (bool, error) {
	// Extract host from server (handle host:port format)
	host := server.Host
	if strings.Contains(host, ":") {
		var err error
		host, _, err = net.SplitHostPort(server.Host)
		if err != nil {
			host = server.Host
		}
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		logFields["error"] = err
		return true, nil // DNS lookup failure shouldn't fail the whole check
	}

	// Check if any resolved IP is IPv6
	hasIPv6 := false
	for _, ip := range ips {
		if ip.To4() == nil && ip.To16() != nil {
			hasIPv6 = true
			break
		}
	}

	server.mu.Lock()
	server.IPv6 = hasIPv6
	server.mu.Unlock()

	if hasIPv6 {
		log.WithField("host", server.Host).Debug("Server has IPv6 support")
	} else {
		logFields["cause"] = "No AAAA record found"
		log.WithField("host", server.Host).Debug("Server does not have IPv6 support")
	}

	// This check doesn't fail servers, it just updates their IPv6 status
	return true, nil
}

type VersionCheck struct {
	config          *Config
	VersionURL      string
	lastVersion     string
	lastVersionTime time.Time
}

func (v *VersionCheck) getCurrentVersion() (string, error) {
	if v.lastVersion != "" && time.Now().Before(v.lastVersionTime.Add(5*time.Minute)) {
		return v.lastVersion, nil
	}

	req, err := http.NewRequest(http.MethodGet, v.VersionURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "ArmbianRouter/1.0 (Go "+runtime.Version()+")")

	res, err := v.config.checkClient.Do(req)

	if err != nil {
		return "", err
	}

	defer res.Body.Close()

	b, err := io.ReadAll(io.LimitReader(res.Body, 128))

	if err != nil {
		return "", err
	}

	v.lastVersion = strings.TrimSpace(string(b))
	v.lastVersionTime = time.Now()

	return v.lastVersion, nil
}

func (v *VersionCheck) Check(server *Server, logFields log.Fields) (bool, error) {
	currentVersion, err := v.getCurrentVersion()

	if err != nil {
		return false, err
	}

	controlPath := path.Join(server.Path, "control")

	u := &url.URL{
		Scheme: "https",
		Host:   server.Host,
		Path:   controlPath,
	}

	if !lo.Contains(server.Protocols, "https") {
		u.Scheme = "http"
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("User-Agent", "ArmbianRouter/1.0 (Go "+runtime.Version()+")")

	res, err := v.config.checkClient.Do(req)

	if err != nil {
		return false, err
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		logFields["error"] = "Control file does not exist"
		return false, nil
	}

	b, err := io.ReadAll(io.LimitReader(res.Body, 128))

	if err != nil {
		return false, err
	}

	actualVersion := strings.TrimSpace(string(b))

	if actualVersion != currentVersion {
		logFields["expectedVersion"] = currentVersion
		logFields["actualVersion"] = actualVersion
		return false, fmt.Errorf("version mismatch, expected: %s, actual: %s", currentVersion, actualVersion)
	}

	return true, nil
}
