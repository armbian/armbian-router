package util

import (
	"crypto/x509"
	"github.com/gwatts/rootcerts/certparse"
	log "github.com/sirupsen/logrus"
	"net/http"
)

const (
	defaultDownloadURL = "https://github.com/mozilla/gecko-dev/blob/master/security/nss/lib/ckfw/builtins/certdata.txt?raw=true"
)

// LoadCACerts loads the certdata from Mozilla and parses it into a CertPool.
func LoadCACerts() (*x509.CertPool, error) {
	res, err := http.Get(defaultDownloadURL)

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	certs, err := certparse.ReadTrustedCerts(res.Body)

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
