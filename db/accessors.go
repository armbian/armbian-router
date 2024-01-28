package db

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	mapIndexRe   = regexp.MustCompile("\\[([a-zA-Z0-9]+)]")
	sliceIndexRe = regexp.MustCompile("\\[(-?\\d+)]")
)

func getMapIndex(key string) string {
	m := mapIndexRe.FindStringSubmatch(key)

	if m == nil {
		return ""
	}

	return m[1]
}

func getSliceIndex(key string) *int {
	m := sliceIndexRe.FindStringSubmatch(key)

	if m == nil {
		return nil
	}

	v, err := strconv.Atoi(m[1])

	if err != nil {
		return nil
	}

	return &v
}

// GetValue is a generated, optimized value getter based on string keys
// This is the only function in this file that should be edited when adding new types.
func GetValue(val any, key string) (any, bool) {
	keysSplit := strings.Split(key, ".")

	switch keysSplit[0] {
	case "asn":
		return getASN(val.(ASN), keysSplit[1:])
	case "city":
		return getCity(val.(City), keysSplit[1:])
	}

	return nil, false
}

func getASN(v ASN, keys []string) (any, bool) {
	key := keys[0]

	switch key {
	case "AutonomousSystemNumber", "autonomous_system_number":
		return v.AutonomousSystemNumber, true
	case "AutonomousSystemOrganization", "autonomous_system_organization":
		return v.AutonomousSystemOrganization, true
	}

	return nil, false
}

func getCity(v City, keys []string) (any, bool) {
	key := keys[0]

	switch key {
	case "Continent", "continent":
		return getContinent(v.Continent, keys[1:])
	case "Country", "country":
		return getCountry(v.Country, keys[1:])
	case "Location", "location":
		return getLocation(v.Location, keys[1:])
	case "RegisteredCountry", "registered_country":
		return getRegisteredCountry(v.RegisteredCountry, keys[1:])
	}

	return nil, false
}

func getContinent(v Continent, keys []string) (any, bool) {
	key := keys[0]

	switch key {
	case "Code", "code":
		return v.Code, true
	case "GeoNameID", "geoname_id":
		return v.GeoNameID, true
	case "Names", "names":
		index := getMapIndex(key)

		if index == "" {
			return nil, false
		}

		m, found := v.Names[index]
		return m, found
	}

	return nil, false
}

func getCountry(v Country, keys []string) (any, bool) {
	key := keys[0]

	switch key {
	case "GeoNameID", "geoname_id":
		return v.GeoNameID, true
	case "IsoCode", "iso_code":
		return v.IsoCode, true
	case "Names", "names":
		index := getMapIndex(key)

		if index == "" {
			return nil, false
		}

		m, found := v.Names[index]
		return m, found
	}

	return nil, false
}

func getLocation(v Location, keys []string) (any, bool) {
	key := keys[0]

	switch key {
	case "AccuracyRadius", "accuracy_radius":
		return v.AccuracyRadius, true
	case "Latitude", "latitude":
		return v.Latitude, true
	case "Longitude", "longitude":
		return v.Longitude, true
	}

	return nil, false
}

func getRegisteredCountry(v RegisteredCountry, keys []string) (any, bool) {
	key := keys[0]

	switch key {
	case "GeoNameID", "geoname_id":
		return v.GeoNameID, true
	case "IsoCode", "iso_code":
		return v.IsoCode, true
	case "Names", "names":
		index := getMapIndex(key)

		if index == "" {
			return nil, false
		}

		m, found := v.Names[index]
		return m, found
	}

	return nil, false
}
