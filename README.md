<h2 align="center">
  <a href=#><img src="https://raw.githubusercontent.com/armbian/.github/master/profile/logosmall.png" alt="Armbian logo"></a>
  <br><br>
</h2>

# Armbian Router (dlrouter)

## Purpose of This Repository

This repository contains the source code for the **Armbian redirector service** (`dlrouter`), which handles intelligent redirection for Armbian OS image downloads and APT package archive access. It routes users to the optimal mirror based on availability, geographic proximity, ASN, and per-server rules, and serves as a central entry point for Armbian's distributed mirror infrastructure.

## Features

- GeoIP + distance-based routing (MaxMind GeoLite2)
- Server weighting and pooling (top N candidates are considered instead of a single one)
- Health checks for HTTP and TLS endpoints
- Optional path remapping (`dl_map`)
- Per-server rules (e.g. ASN allow/deny, country allow/deny)
- Prometheus metrics endpoint
- SVG status badges for dynamic mirror lists

## Built With

- **Go 1.21** (module `github.com/armbian/redirector`)
- HTTP routing with **go-chi/chi v5** and `chi-middleware/logrus-logger`
- Configuration via **spf13/viper** (YAML)
- Logging with **sirupsen/logrus**
- GeoIP lookups via **oschwald/maxminddb-golang**
- LRU caching via **hashicorp/golang-lru**
- Root CA bundle via **gwatts/rootcerts** (Mozilla CA list)
- Concurrency helpers via **sourcegraph/conc**
- Metrics via **prometheus/client_golang**
- Testing with **Ginkgo v2** and **Gomega**
- Container image built from a **distroless** base (see `Dockerfile`)

## Repository Layout

```
.
├── cmd/
│   ├── main.go           # Service entry point
│   └── db/genaccessors.go
├── db/                   # GeoIP structs and generated accessors
│   ├── accessors.go
│   └── structs.go
├── middleware/           # HTTP middleware
│   └── middleware.go
├── util/                 # Certificate and helper utilities
│   ├── certificates.go
│   └── util.go
├── assets/               # SVG status badges (up/down/unknown)
├── check.go              # Health check logic
├── config.go             # Configuration loading
├── http.go               # HTTP handlers
├── map.go                # Path/dl_map handling
├── mirrors.go            # Mirror list handling
├── redirector.go         # Redirect logic
├── servers.go            # Server pool + selection
├── dlrouter.yaml         # Example configuration
├── Dockerfile
├── go.mod / go.sum
└── *_test.go             # Ginkgo/Gomega test suites
```

## Building

### From source

```sh
go build -o dlrouter ./cmd/main.go
```

### With Docker

```sh
docker build -t armbian-router .
docker run --rm -p 8080:8080 -v $PWD/dlrouter.yaml:/dlrouter.yaml armbian-router
```

A prebuilt image is published to `ghcr.io/armbian/armbian-router:latest`.

## Configuration

Configuration is loaded via Viper (YAML). See `dlrouter.yaml` for a working example.

### Modes

#### Redirect
Standard redirect functionality: incoming requests are routed to the best available mirror.

#### Download Mapping
When `dl_map` is set, the referenced CSV file is used to remap request paths to new paths — conceptually like symlinks in a generated file.

### Example configuration

```yaml
# GeoIP Database Path
geodb: GeoLite2-City.mmdb

# Comment out to disable
dl_map: userdata.csv

# LRU Cache Size (in items)
cacheSize: 1024

# Server definition
# Weights behave like nginx: if > 1, chosen weight/(weight+total) of the time.
# By default the top 3 servers are considered for best-match selection.
servers:
  - server: armbian.12z.eu/apt/
  - server: armbian.chi.auroradev.org/apt/
    weight: 15
    latitude: 41.8879
    longitude: -88.1995
  # A mirror advertising additional protocols (e.g. rsync)
  - server: mirrors.dotsrc.org/armbian-apt/
    weight: 15
    protocols:
      - http
      - https
      - rsync
  # Example of a mirror with rules
  - server: armbian.lv.auroradev.org/apt/
    rules:
      # field + one of: is, is_not, in, not_in
      - field: asn.autonomous_system_number
        is_not: 15169
      - field: location.country.iso_code
        not_in:
          - RU
```

## Health Checks

### HTTP
Verifies server accessibility over HTTP. If a server force-redirects to `https://`, it is considered HTTPS-only. If the HTTPS endpoint force-redirects back to HTTP, the mirror is marked down (requests must not downgrade).

### TLS
Certificate validation using the Mozilla CA list (loaded on startup and on config reload), rather than the OS trust store. The bundle is fetched from GitHub at startup/reload.

## API

| Endpoint | Description |
| --- | --- |
| `/status` | Simple health check (useful for upstream proxies such as nginx). |
| `/reload` | Flushes cache and reloads configuration/mapping. Requires `reloadToken` in configuration and a matching `Authorization: Bearer TOKEN` header. |
| `/mirrors` | All mirrors in the legacy (region-grouped) format. |
| `/mirrors.json` | All mirrors in the new JSON format (see below). |
| `/mirrors/{server}.svg` | Status badge for a given server, for use in dynamic mirror lists. |
| `/dl_map` | JSON-encoded download mappings. |
| `/geoip` | GeoIP information for the requester. |
| `/region/{REGIONCODE}/{PATH}` | Force redirection to a region: `NA`, `EU`, or `AS`. |
| `/metrics` | Prometheus metrics (public). |

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

## Testing

Tests use Ginkgo v2 and Gomega. To run them:

```sh
go install github.com/onsi/ginkgo/v2/ginkgo@latest
ginkgo --randomize-all --p --cover --coverprofile=cover.out .
go tool cover -func=cover.out
```

See `check_test.go` and `map_test.go` for examples.

## Continuous Integration

An overview of this repository's CI status and pipelines is available at:

<https://actions.armbian.com/?repo=armbian-router>

## Contributing

Contributions are welcome. The codebase is being progressively cleaned up, standardized, and covered with tests — pull requests improving any of these areas are especially appreciated. See `check_test.go` for example tests you can model new ones on.

## License

Released under the ISC license. See [`LICENSE`](LICENSE) for details.  
Copyright (c) 2022 Tyler Stuyfzand and the Armbian Project.

## Related Links

- Armbian website: <https://www.armbian.com>
- Armbian documentation: <https://docs.armbian.com>
