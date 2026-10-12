<h2 align="center">
  <a href=#><img src="https://raw.githubusercontent.com/armbian/.github/master/profile/logosmall.png" alt="Armbian logo"></a>
  <br><br>
</h2>

# Armbian Router (dlrouter)

## Purpose of This Repository

This repository contains the source code for the **Armbian redirector service** (`dlrouter`), which performs intelligent redirection for Armbian OS image downloads and APT package archive access. It routes clients to an optimal mirror based on geographic proximity, server weighting, health checks and configurable rules.

## Features

- GeoIP + distance-based routing (MaxMind GeoLite2)
- Server weighting and pooling (top-N candidate selection, like nginx weights)
- Health checks over HTTP and TLS
- Path mapping via a CSV download map (symlink-like rewrites)
- Per-server protocols (e.g. `http`, `https`, `rsync`) and rule-based filters (ASN, country, ...)
- Status badges (SVG) per mirror
- Prometheus `/metrics` endpoint
- JSON and legacy mirror listing endpoints

## Built With

- **Go** (module `github.com/armbian/redirector`, Go 1.21)
- [go-chi/chi](https://github.com/go-chi/chi) HTTP router with logrus logging middleware
- [spf13/viper](https://github.com/spf13/viper) for YAML configuration
- [oschwald/maxminddb-golang](https://github.com/oschwald/maxminddb-golang) for GeoIP lookups
- [gwatts/rootcerts](https://github.com/gwatts/rootcerts) for the Mozilla CA bundle
- [hashicorp/golang-lru](https://github.com/hashicorp/golang-lru) LRU cache
- [prometheus/client_golang](https://github.com/prometheus/client_golang) metrics
- [Ginkgo v2](https://github.com/onsi/ginkgo) + [Gomega](https://github.com/onsi/gomega) tests
- Docker (multi-stage build, distroless runtime)

## Repository Layout

```
.
├── cmd/
│   ├── main.go           # Service entry point (built as `dlrouter`)
│   └── db/
│       └── genaccessors.go
├── db/
│   ├── accessors.go      # Generated GeoIP record accessors
│   └── structs.go
├── middleware/
│   └── middleware.go
├── util/
│   ├── certificates.go   # Mozilla CA bundle handling
│   └── util.go
├── assets/               # status-up/down/unknown SVG badges
├── check.go              # HTTP/TLS health checks
├── config.go             # Viper-based config loading
├── http.go               # HTTP server & routes
├── map.go                # Download path mapping (dl_map)
├── mirrors.go            # Mirror listings
├── redirector.go         # Core redirection logic
├── servers.go            # Server pool, weighting, scoring
├── dlrouter.yaml         # Example configuration
├── Dockerfile
└── go.mod
```

## Building

Requires Go 1.21+.

```sh
go build -o dlrouter ./cmd/main.go
```

### Docker

A multi-stage Dockerfile is provided; the final image is based on `gcr.io/distroless/static:nonroot`.

```sh
docker build -t armbian-router .
docker run --rm -p 8080:8080 -v $(pwd)/dlrouter.yaml:/dlrouter.yaml armbian-router
```

Published images are available from GitHub Container Registry at `ghcr.io/armbian/armbian-router`.

## Testing

Tests use Ginkgo v2 and Gomega. See `check_test.go`, `map_test.go` and `armbianmirror_suite_test.go` for examples.

```sh
go install github.com/onsi/ginkgo/v2/ginkgo
ginkgo --randomize-all --p --cover --coverprofile=cover.out .
go tool cover -func=cover.out
```

A vet pass is also recommended:

```sh
go vet .
```

## Configuration

Configuration is loaded from a YAML file (see `dlrouter.yaml`).

### Modes

#### Redirect

Standard redirect functionality.

#### Download Mapping

Uses the `dl_map` configuration variable to enable mapping of paths to new paths — think symlinks, but in a generated file.

### Mirrors

Mirror targets (with trailing slash) are declared in the YAML configuration.

### Example YAML

```yaml
# GeoIP Database Path
geodb: GeoLite2-City.mmdb

# Comment out to disable
dl_map: userdata.csv

# LRU Cache Size (in items)
cacheSize: 1024

# Server definition
# Weights are just like nginx: if > 1 it'll be chosen x out of x + total times.
# By default, the top 3 servers are used for choosing the best.
# server    = full url or host+path
# weight    = int
# optional: latitude, longitude (float)
# optional: protocols (list/array)
servers:
  - server: armbian.12z.eu/apt/
  - server: armbian.chi.auroradev.org/apt/
    weight: 15
    latitude: 41.8879
    longitude: -88.1995
  # Server with additional protocols (e.g. rsync sources)
  - server: mirrors.dotsrc.org/armbian-apt/
    weight: 15
    protocols:
      - http
      - https
      - rsync
  # Server with routing rules
  - server: armbian.lv.auroradev.org/apt/
    rules:
      # field + value matcher: is, is_not, in, not_in
      - field: asn.autonomous_system_number
        is_not: 15169
      - field: location.country.iso_code
        not_in:
          - RU
```

## Health Checks

### HTTP

Verifies server accessibility via HTTP. If the server returns a forced redirect to an `https://` URL, it is considered HTTPS-only. If an `https` URL responds with a forced downgrade to `http`, the mirror is marked down — requests should never downgrade.

### TLS

Certificate validation uses the Mozilla CA certificate list, loaded on startup/reload, instead of the OS trust store (which caused some date-validation inconsistencies in practice). The CA bundle is fetched from GitHub on each start/reload.

## HTTP API

| Path | Purpose |
|------|---------|
| `/status` | Simple health check (for upstream proxies like nginx) |
| `/reload` | Flushes cache and reloads config/mapping. Requires `reloadToken` in config and matching `Authorization: Bearer TOKEN` |
| `/mirrors` | Legacy mirror listing (by region) |
| `/mirrors.json` | JSON mirror listing |
| `/mirrors/{server}.svg` | Status badge SVG for a given server |
| `/dl_map` | JSON-encoded download mappings |
| `/geoip` | GeoIP information for the requesting client |
| `/region/{REGIONCODE}/{PATH}` | Force redirect to a region (`NA`, `EU`, `AS`) |
| `/metrics` | Prometheus metrics (public) |

Example `/mirrors.json` entry:

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

## Code Quality

Work is ongoing to clean up and standardize the code, add tests, and improve consistency. Contributions are welcome — see `check_test.go` for test examples.

## Continuous Integration

For an overview of this repository's CI workflows and their current status, see the Armbian CI dashboard:

<https://actions.armbian.com/?repo=armbian-router>

## License

Released under the ISC license. See [LICENSE](LICENSE).

Copyright (c) 2022 Tyler Stuyfzand <admin@meow.tf>, Armbian Project.

## Related Links

- Armbian: <https://www.armbian.com>
- Documentation: <https://docs.armbian.com>
