package geo

import (
	"net"
)

type Provider interface {
	City(ip net.IP) (*City, error)
	ASN(ip net.IP) (*ASN, error)
	Close() error
}
