package geo

// City represents a MaxmindDB city.
// This used to only be used on load, but is now used with rules as well.
type City struct {
	Continent         Continent         `maxminddb:"continent" json:"continent"`
	Country           Country           `maxminddb:"country" json:"country"`
	Location          Location          `maxminddb:"location"`
	RegisteredCountry RegisteredCountry `maxminddb:"registered_country" json:"registered_country"`
}

type Continent struct {
	Code      string            `maxminddb:"code" json:"code"`
	GeoNameID uint              `maxminddb:"geoname_id" json:"geoname_id"`
	Names     map[string]string `maxminddb:"names" json:"names"`
}

type Country struct {
	GeoNameID uint              `maxminddb:"geoname_id" json:"geoname_id"`
	IsoCode   string            `maxminddb:"iso_code" json:"iso_code"`
	Names     map[string]string `maxminddb:"names" json:"names"`
}

type Location struct {
	AccuracyRadius uint16  `maxminddb:"accuracy_radius" json:"accuracy_radius"`
	Latitude       float64 `maxminddb:"latitude" json:"latitude"`
	Longitude      float64 `maxminddb:"longitude" json:"longitude"`
}

type RegisteredCountry struct {
	GeoNameID uint              `maxminddb:"geoname_id" json:"geoname_id"`
	IsoCode   string            `maxminddb:"iso_code" json:"iso_code"`
	Names     map[string]string `maxminddb:"names" json:"names"`
}

// The ASN struct corresponds to the data in the GeoLite2 ASN database.
type ASN struct {
	AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}
