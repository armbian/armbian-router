package geo

import (
	"fmt"
	"github.com/oschwald/maxminddb-golang"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"net"
)

var ErrNoASN = errors.New("no asn database loaded")

type MaxmindProvider struct {
	db    *maxminddb.Reader
	asnDB *maxminddb.Reader
}

func (m *MaxmindProvider) City(ip net.IP) (*City, error) {
	var city City

	if err := m.db.Lookup(ip, &city); err != nil {
		return nil, err
	}

	return &city, nil
}

func (m *MaxmindProvider) ASN(ip net.IP) (*ASN, error) {
	if m.asnDB == nil {
		return nil, ErrNoASN
	}

	var asn ASN

	if err := m.asnDB.Lookup(ip, &asn); err != nil {
		log.WithError(err).Warning("Unable to load ASN information")
		return nil, err
	}

	return &asn, nil
}

func (m *MaxmindProvider) Close() error {
	if m.db != nil {
		_ = m.db.Close()
	}

	if m.asnDB != nil {
		_ = m.asnDB.Close()
	}

	return nil
}

func NewMaxmindProvider(geoPath, asnPath string) (Provider, error) {
	// db can be hot-reloaded if the file changed
	db, err := maxminddb.Open(geoPath)

	if err != nil {
		return nil, fmt.Errorf("unable to open geo database: %w", err)
	}

	var asnDB *maxminddb.Reader

	if asnPath != "" {
		asnDB, err = maxminddb.Open(asnPath)
		if err != nil {
			return nil, fmt.Errorf("unable to open asn database: %w", err)
		}
	}

	return &MaxmindProvider{
		db:    db,
		asnDB: asnDB,
	}, nil
}
