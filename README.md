<h2 align="center">
  <a href=#><img src="https://raw.githubusercontent.com/armbian/.github/master/profile/logosmall.png" alt="Armbian logo"></a>
  <br><br>
</h2>

# Armbian Router (dlrouter)

## Purpose of This Repository

This repository contains the source code for the **Armbian redirector service** (`dlrouter`), which handles intelligent redirection for Armbian OS image downloads and APT package archive access. It routes users to the optimal mirror based on availability, geographic proximity, protocol, and configurable rules, and acts as a central entry point for Armbian's distributed download infrastructure.

## Features

- GeoIP + distance-based routing (MaxMind GeoLite2)
- Server weighting and pooling (the top-N closest/best servers are considered, not just one)
- Health checks over HTTP and TLS
- Download path mapping (CSV-driven path rewrites)
- Rule-based mirror selection (e.g., ASN or country filtering)
- Per-server protocol declarations (`http`, `https`, `rsync`, ...)
- Prometheus metrics endpoint
- SVG status badges for use in dynamic mirror lists

## Built With

- **Go** (module `github.com/armbian/redirector`, Go 1.21)
- HTTP routing via `go-chi/chi`
- Config via `spf13/viper` (YAML)
- Logging via `sirupsen/logrus`
- Testing via `onsi/ginkgo/v2` and `onsi/gomega`
- MaxMind DB reader (`oschwald/maxminddb-golang`)
- Mozilla CA roots via `gwatts/rootcerts`
- LRU caching via `hashicorp/golang-lru`
- Prometheus client (`prometheus/client_golang`)
- Container image built on `golang:alpine` → `gcr.io/distroless/static:nonroot`

## Repository Layout

```
.
├── cmd/
│   ├── main.go              # Service entry point
│   └── db/genaccessors.go   # Code generator for db accessors
├── db/
│   ├── accessors.go
│   └── structs.go           # GeoIP / rule input structs
├── middleware/
│   └── middleware.go
├── util/
│   ├── certificates.go      # CA bundle handling
│   └── util.go
├── assets/                  # SVG status badges (up / down / unknown)
├── check.go / check_test.go # HTTP + TLS health checks
├── config.go                # Configuration loading
├── http.go                  # HTTP handlers
├── map.go / map_test.go     # Download path mapping
├── mirrors.go               # Mirror list handling
├── redirector.go            # Core redirect logic
├── servers.go               # Server pool / weighting / selection
├── armbianmirror_suite_test.go
├── dlrouter.yaml            # Example configuration
├── Dockerfile
├── go.mod / go.sum
└── .drone.yml
```

## Building

### From source

```sh
go build -o dlrouter ./cmd/main.go
```

### Docker

```sh
docker build -t dlrouter .
docker run --rm -p 8080:8080 -v $PWD/dlrouter.yaml:/dlrouter.yaml dlrouter
```

A prebuilt image is published to GitHub Container Registry at `ghcr.io/armbian/armbian-router:latest`.

## Running Tests

Tests use the Ginkgo v2 framework:

```sh
go install github.com/onsi/ginkgo/v2/ginkgo
ginkgo --randomize-all --p --cover --coverprofile=cover.out .
go tool cover -func=cover.out
```

`go vet ./...` is also run in CI.

## Configuration

Configuration is loaded from a YAML file (see `dlrouter.yaml` in this repo for a working example).

### Modes

#### Redirect

Standard mirror redirect functionality based on client location, server weight and health.

#### Download Mapping

Enabled by setting the `dl_map` configuration variable to a CSV file. This maps request paths to new paths — think symlinks, but in a generated file.

### Mirrors

Mirror targets (with trailing slash) are listed in the YAML configuration.

### Example YAML

```yaml
# GeoIP Database Path
geodb: GeoLite2-City.mmdb

# Comment out to disable
dl_map: userdata.csv

# LRU Cache Size (in items)
cacheSize: 1024

# Server definition
# Weights are just like nginx, where if it's > 1 it'll be chosen x out of x + total times
# By default, the top 3 servers are used for choosing the best.
# server = full url or host+path
# weight = int
# optional: latitude, longitude (float)
# optional: protocols (list/array)
servers:
  - server: armbian.12z.eu/apt/
  - server: armbian.chi.auroradev.org/apt/
    weight: 15
    latitude: 41.8879
    longitude: -88.1995
  # Example of a server with additional protocols (rsync)
  - server: mirrors.dotsrc.org/armbian-apt/
    weight: 15
    protocols:
      - http
      - https
      - rsync
  # Example of a server with rules
  - server: armbian.lv.auroradev.org/apt/
    rules:
      # Value matchers: is, is_not, in, not_in
      - field: asn.autonomous_system_number
        is_not: 15169
      - field: location.country.iso_code
        not_in:
          - RU
```

## Health Checks

### HTTP

Verifies server accessibility via HTTP. If the server returns a forced redirect to an `https://` URL, it is considered HTTPS-only. If the server responds on the HTTPS URL with a forced HTTP redirect, it is marked down due to misconfiguration — requests should never downgrade.

### TLS

Verifies that server certificates are valid and not expired. The Mozilla CA certificate list is loaded on start / config reload and used to verify roots. OS trust stores were previously used, but switching to the bundled Mozilla list proved more reliable.

> Note: the CA bundle is fetched on every startup/reload.

## HTTP API

| Endpoint | Description |
|---|---|
| `/status` | Simple health check (suitable for upstream HTTP checks). |
| `/reload` | Flushes cache and reloads config/mapping. Requires `reloadToken` in config, matched via `Authorization: Bearer TOKEN`. |
| `/mirrors` | All mirrors in the legacy (by region) format. |
| `/mirrors.json` | All mirrors in JSON format. |
| `/mirrors/{server}.svg` | SVG badge reflecting a given server's current status. |
| `/dl_map` | JSON-encoded download mapping. |
| `/geoip` | GeoIP information for the requester. |
| `/region/{REGIONCODE}/{PATH}` | Redirect into a specific region (`NA`, `EU`, `AS`). |
| `/metrics` | Prometheus metrics (public). |

### `/mirrors.json` example

```json
[
  {
    "available": true,
    "host": "imola.armbian.com",
    "path": "/apt/",
    "latitude": 46.0503,
    "longitude": 14.5046,
    "weight": 10,
    "continent": "EU",
    "lastChange": "2022-08-12T06:52:35.029565986Z"
  }
]
```

## Continuous Integration

For an overview of this repository's CI pipelines and their current status, see the Armbian CI dashboard:

<https://actions.armbian.com/?repo=armbian-router>

## Code Quality & Contributing

The codebase is actively being cleaned up, standardized, and expanded with tests. Contributions are very welcome — see `check_test.go` for examples of how tests are structured in this project.

## Related

- Armbian website: <https://www.armbian.com>
- Armbian documentation: <https://docs.armbian.com>

## License

Released under the ISC license. See [LICENSE](LICENSE) for the full text.
