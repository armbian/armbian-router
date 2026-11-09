<h2 align="center">
  <a href=#><img src="https://raw.githubusercontent.com/armbian/.github/master/profile/logosmall.png" alt="Armbian logo"></a>
  <br><br>
</h2>

# Armbian Router (dlrouter)

## Purpose of This Repository

This repository contains the source code for the **Armbian redirector service** (`dlrouter`), which performs intelligent redirection for Armbian OS image downloads and APT package archive access. It routes users to an appropriate mirror based on availability, geographic proximity, server weighting, and per-mirror rules.

## Features

- Written in Go (module `github.com/armbian/redirector`, Go 1.21)
- GeoIP + distance-based routing using MaxMind databases
- Server weighting and pooling (top N servers are considered rather than a single one)
- Health checks for HTTP and TLS endpoints
- Download path remapping via a CSV map file (symlink-like behavior in a generated file)
- Prometheus metrics endpoint
- Tests built on Ginkgo v2 and Gomega

## Tech Stack

- **Language:** Go (`.go` sources under the repository root, `cmd/`, `db/`, `middleware/`, `util/`)
- **HTTP routing:** [`go-chi/chi`](https://github.com/go-chi/chi) with `chi-middleware/logrus-logger`
- **Configuration:** [`spf13/viper`](https://github.com/spf13/viper) (YAML, see `dlrouter.yaml`)
- **GeoIP:** [`oschwald/maxminddb-golang`](https://github.com/oschwald/maxminddb-golang)
- **TLS roots:** [`gwatts/rootcerts`](https://github.com/gwatts/rootcerts) (Mozilla CA bundle)
- **Caching:** [`hashicorp/golang-lru`](https://github.com/hashicorp/golang-lru)
- **Metrics:** [`prometheus/client_golang`](https://github.com/prometheus/client_golang)
- **Logging:** [`sirupsen/logrus`](https://github.com/sirupsen/logrus)
- **Testing:** [`onsi/ginkgo/v2`](https://github.com/onsi/ginkgo) + [`onsi/gomega`](https://github.com/onsi/gomega)
- **Container image:** multi-stage `Dockerfile` based on `golang:alpine` → `gcr.io/distroless/static:nonroot`

## Repository Layout

```
.
├── cmd/
│   ├── main.go              # Service entry point
│   └── db/genaccessors.go   # Code generator for db accessors
├── db/
│   ├── accessors.go         # Generated accessors
│   └── structs.go           # GeoIP/record structs
├── middleware/
│   └── middleware.go        # HTTP middleware
├── util/
│   ├── certificates.go      # Mozilla CA bundle handling
│   └── util.go
├── assets/
│   ├── status-up.svg
│   ├── status-down.svg
│   └── status-unknown.svg
├── check.go / check_test.go # Mirror health checks
├── config.go                # Configuration loading
├── http.go                  # HTTP server & routes
├── map.go / map_test.go     # Download path mapping
├── mirrors.go               # Mirrors data model
├── redirector.go            # Redirect logic
├── servers.go               # Server selection
├── armbianmirror_suite_test.go
├── dlrouter.yaml            # Example configuration
├── Dockerfile
└── go.mod / go.sum
```

## Building

### From source

Requires Go 1.21 or newer.

```sh
go build -o dlrouter ./cmd/main.go
```

### With Docker

```sh
docker build -t dlrouter .
docker run --rm -p 8080:8080 -v "$PWD/dlrouter.yaml:/dlrouter.yaml" dlrouter
```

A prebuilt image is also published to GHCR at `ghcr.io/armbian/armbian-router:latest`.

## Running Tests

The suite uses Ginkgo:

```sh
go install github.com/onsi/ginkgo/v2/ginkgo@latest
ginkgo --randomize-all --p --cover --coverprofile=cover.out .
go tool cover -func=cover.out
```

See `check_test.go` for example tests.

## Checks

The supported health checks are **HTTP** and **TLS**.

### HTTP

Verifies server accessibility via HTTP. If the server returns a forced redirect to an `https://` URL, it is considered HTTPS-only. If the server responds on the HTTPS URL with a forced HTTP redirect, it is marked down due to misconfiguration — requests should never downgrade.

### TLS

Certificate checking ensures no servers are used which have invalid/expired certificates. This check uses the Mozilla CA certificate list (loaded on start and on config reload) to verify roots, rather than the OS certificate trust store. Note: the bundle is fetched from GitHub on each startup/reload.

## Configuration

### Modes

#### Redirect

Standard redirect functionality.

#### Download Mapping

Uses the `dl_map` configuration variable to enable mapping of paths to new paths — like symlinks, but in a generated file.

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
      # This example excludes Google's ASN from this mirror
      - field: asn.autonomous_system_number
        is_not: 15169
      # Country blocking example
      - field: location.country.iso_code
        not_in:
          - RU
```

## HTTP API

| Endpoint | Description |
| --- | --- |
| `/status` | Simple health check (for nginx/LB 502 fallback). |
| `/reload` | Flushes cache and reloads configuration/mapping. Requires `reloadToken` in the config and a matching `Authorization: Bearer TOKEN` header. |
| `/mirrors` | All mirrors in the legacy (by region) format. |
| `/mirrors.json` | All mirrors in the new JSON format. |
| `/mirrors/{server}.svg` | SVG badge showing the server's status, for dynamic mirror lists. |
| `/dl_map` | JSON-encoded download mappings. |
| `/geoip` | GeoIP information for the requester. |
| `/region/{REGIONCODE}/{PATH}` | Redirect into a specific region (`NA`, `EU`, `AS`). |
| `/metrics` | Prometheus metrics endpoint (public). |

Example `/mirrors.json` response:

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

An overview of this repository's CI pipelines is available on the Armbian CI dashboard:

<https://actions.armbian.com/?repo=armbian-router>

## Code Quality & Contributing

Work is ongoing to clean up and standardize the codebase and improve test coverage. Contributions are welcome — see `check_test.go` for examples of how tests are structured.

## License

Released under the ISC license. See [LICENSE](LICENSE) for details.

Copyright (c) 2022 Tyler Stuyfzand &lt;admin@meow.tf&gt;, Armbian Project.

## Related Links

- Armbian: <https://www.armbian.com>
- Documentation: <https://docs.armbian.com>
