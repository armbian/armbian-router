package redirector

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
	"strings"
	"time"
)

var (
	ErrHttpsRedirect = errors.New("unexpected forced https redirect")
	ErrCertExpired   = errors.New("certificate is expired")
)

func (r *Redirector) checkHttp(scheme string) ServerCheck {
	return func(server *Server, logFields log.Fields) (bool, error) {
		return r.checkHttpScheme(server, scheme, logFields)
	}
}

// checkHttp checks a URL for validity, and checks redirects
func (r *Redirector) checkHttpScheme(server *Server, scheme string, logFields log.Fields) (bool, error) {
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

			switch u.Scheme {
			case "http":
				res, err := r.checkRedirect(u.Scheme, location)

				if !res || err != nil {
					// If we don't support http, we remove it from supported protocols
					server.Protocols = server.Protocols.Remove("http")
				} else {
					// Otherwise, we verify https support
					r.checkProtocol(server, "https")
				}
			case "https":
				// We don't want to allow downgrading, so this is an error.
				return r.checkRedirect(u.Scheme, location)
			}
		}

		return true, nil
	}

	logFields["cause"] = fmt.Sprintf("Unexpected http status %d", res.StatusCode)

	return false, nil
}

func (r *Redirector) checkProtocol(server *Server, scheme string) {
	res, err := r.checkHttpScheme(server, scheme, log.Fields{})

	if !res || err != nil {
		return
	}

	if !server.Protocols.Contains(scheme) {
		server.Protocols = server.Protocols.Append(scheme)
	}
}

// checkRedirect parses a location header response and checks the scheme
func (r *Redirector) checkRedirect(originatingScheme, locationHeader string) (bool, error) {
	newUrl, err := url.Parse(locationHeader)

	if err != nil {
		return false, err
	}

	if newUrl.Scheme == "https" {
		return false, ErrHttpsRedirect
	} else if originatingScheme == "https" && newUrl.Scheme == "https" {
		return false, ErrHttpsRedirect
	}

	return true, nil
}

// checkTLS checks tls certificates from a host, ensures they're valid, and not expired.
func (r *Redirector) checkTLS(server *Server, logFields log.Fields) (bool, error) {
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
	}).Info("Checking TLS server")

	if port == "" {
		port = "443"
	}

	conn, err := tls.Dial("tcp", host+":"+port, &tls.Config{
		RootCAs: r.config.RootCAs,
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
		Roots:         r.config.RootCAs,
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
	if !server.Protocols.Contains("https") {
		server.Protocols = server.Protocols.Append("https")
	}

	return true, nil
}
