package geo

import (
	"github.com/stretchr/testify/mock"
	"net"
)

type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) City(ip net.IP) (*City, error) {
	args := m.Mock.Called(ip)

	if v := args.Get(0); v != nil {
		return v.(*City), args.Error(1)
	}

	return nil, args.Error(1)
}

func (m *MockProvider) ASN(ip net.IP) (*ASN, error) {
	args := m.Mock.Called(ip)

	if v := args.Get(0); v != nil {
		return v.(*ASN), args.Error(1)
	}

	return nil, args.Error(1)
}

func (m *MockProvider) Close() error {
	return nil
}
