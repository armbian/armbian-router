package util

import (
	"bytes"
	"crypto/x509"
	"io"
	"net/http"
	"os"

	"github.com/gwatts/rootcerts/certparse"
	log "github.com/sirupsen/logrus"
)

const (
	defaultDownloadURL = "https://raw.githubusercontent.com/mozilla/gecko-dev/refs/heads/master/security/nss/lib/ckfw/builtins/certdata.txt"
)

// LoadCACerts loads the certdata from Mozilla and parses it into a CertPool.
func LoadCACerts(certPath string) (*x509.CertPool, error) {
	var certContents io.Reader

	if certPath != "" {
		res, err := os.ReadFile(certPath)

		if err != nil {
			return nil, err
		}

		certContents = io.NopCloser(bytes.NewReader(res))
	} else {

		res, err := http.Get(defaultDownloadURL)

		if err != nil {
			return nil, err
		}

		defer res.Body.Close()

		certContents = res.Body
	}

	certs, err := certparse.ReadTrustedCerts(certContents)

	if err != nil {
		return nil, err
	}

	pool := x509.NewCertPool()

	var count int

	for _, cert := range certs {
		if cert.Trust&certparse.ServerTrustedDelegator == 0 {
			continue
		}

		count++

		pool.AddCert(cert.Cert)
	}

	log.WithField("certs", count).Info("Loaded root cas")

	return pool, nil
}
