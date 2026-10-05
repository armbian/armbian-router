<h2 align="center">
  <a href=#><img src="https://raw.githubusercontent.com/armbian/.github/master/profile/logosmall.png" alt="Armbian logo"></a>
  <br><br>
</h2>

# Armbian Router (dlrouter)

## Purpose of This Repository

This repository contains the source code for the **Armbian redirector service** (`dlrouter`), which intelligently redirects users to the optimal mirror for Armbian OS image downloads and APT package archive access. It routes requests based on geographic proximity, server weight, availability, and configurable per-mirror rules.

## Features

- GeoIP + distance-based routing (MaxMind GeoLite2)
- Weighted server pooling (top-N candidates served instead of a single one)
- Health checks (HTTP, TLS)
- Per-mirror rules (e.g. ASN or country allow/deny)
- Optional download path mapping (`dl_map`, think symlinks in a generated file)
- Prometheus metrics endpoint
- SVG status badges for dynamic mirror lists

## Built With

- **Go** (module `github.com/armbian/redirector`, Go 1.21)
- HTTP routing via [`go-chi/chi`](https://github.com/go-chi/chi) with `chi-middleware/logrus-logger`
- Configuration via [`spf13/viper`](https://github.com/spf13/viper) (YAML)
- GeoIP lookups via [`oschwald/maxminddb-golang`](https://github.com/oschwald/maxminddb-golang)
- Logging via [`sirupsen/logrus`](https://github.com/sirupsen/logrus)
- Prometheus client (`prometheus/client_golang`)
- Trusted roots via [`gwatts/rootcerts`](https://github.com/gwatts/rootcerts) (Mozilla CA bundle)
- LRU cache (`hashicorp/golang-lru`)
- Testing with **Ginkgo v2** and **Gomega**
- Container image: multi-stage `Dockerfile` on `golang:alpine` → `gcr.io/distroless/static:nonroot`

## Repository Layout

```
.
├── cmd/
│   ├── main.go              # Service entry point
│   └── db/genaccessors.go   # Code generator for db accessors
├── db/
│   ├── accessors.go
│   └── structs.go
├── middleware/
│   └── middleware.go
├── util/
│   ├── certificates.go
│   └── util.go
├── assets/                  # Status SVG badges (up/down/unknown)
├── check.go / check_test.go # Mirror health checks (HTTP/TLS)
├── config.go
├── http.go
├── map.go / map_test.go
├── mirrors.go
├── redirector.go
├── servers.go
├── armbianmirror_suite_test.go
├── dlrouter.yaml            # Example configuration
├── Dockerfile
└── go.mod / go.sum
```

## Building

Build the binary locally with Go:

```sh
go build -o dlrouter ./cmd/main.go
```

Or build the container image:

```sh
docker build -t armbian-router .
```

A pre-built image is published to GitHub Container Registry at `ghcr.io/armbian/armbian-router:latest`.

## Running Tests

Tests use Ginkgo v2:

```sh
go install github.com/onsi/ginkgo/v2/ginkgo
ginkgo --randomize-all --p --cover --coverprofile=cover.out .
go tool cover -func=cover.out
```

Contributions are welcome; see `check_test.go` for example tests.

## Checks

Supported mirror checks are **HTTP** and **TLS**.

### HTTP

Verifies server accessibility via HTTP. If the server returns a forced redirect to an `https://` URL, it is considered HTTPS-only.

If the server responds on an `https` URL with a forced `http` redirect, it will be marked down due to misconfiguration — requests should never downgrade.

### TLS

Certificate validation is performed against the **Mozilla CA bundle**, loaded on start/reload (via `gwatts/rootcerts`), instead of the OS trust store. This avoids inconsistencies observed with date validation on some hosts.

> Note: the CA bundle is fetched on each startup/reload.

## Configuration

### Modes

#### Redirect

Standard redirect functionality.

#### Download Mapping

Uses the `dl_map` configuration variable to enable mapping of paths to new paths — think symlinks, but in a generated file.

### Mirrors

Mirror targets (with trailing slash) are placed in the YAML configuration file. See `dlrouter.yaml` for an example.

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
# By default the top 3 servers are used for choosing the best.
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
  # Server with additional protocols (e.g. rsync)
  - server: mirrors.dotsrc.org/armbian-apt/
    weight: 15
    protocols:
      - http
      - https
      - rsync
  # Server with rules
  - server: armbian.lv.auroradev.org/apt/
    rules:
      # Value matchers: is, is_not, in, not_in
      # Exclude Google's ASN from this mirror
      - field: asn.autonomous_system_number
        is_not: 15169
      # Country-based blocking
      - field: location.country.iso_code
        not_in:
          - RU
```

## HTTP API

| Path | Description |
| --- | --- |
| `/status` | Simple health check endpoint. |
| `/reload` | Flushes cache and reloads configuration/mapping. Requires `reloadToken` in config and a matching `Authorization: Bearer TOKEN`. |
| `/mirrors` | Lists all mirrors in the legacy (by region) format. |
| `/mirrors.json` | Lists all mirrors in the JSON format (see example below). |
| `/mirrors/{server}.svg` | SVG status badge for a given server, for use in dynamic mirror lists. |
| `/dl_map` | JSON-encoded download mappings. |
| `/geoip` | GeoIP information for the requester. |
| `/region/{REGIONCODE}/{PATH}` | Redirects to the desired region (`NA`, `EU`, `AS`). |
| `/metrics` | Prometheus metrics endpoint (publicly exposed). |

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

## Continuous Integration

For an overview of the CI pipelines run against this repository, see the Armbian CI dashboard:

<https://actions.armbian.com/?repo=armbian-router>

Tagged releases (`v*`) also publish a multi-arch (`linux/amd64`, `linux/arm64`) Docker image to GitHub Container Registry.

## License

Released under the ISC license. Copyright (c) 2022 Tyler Stuyfzand and the Armbian Project. See [`LICENSE`](LICENSE) for details.

## Related Links

- Armbian website: <https://www.armbian.com>
- Armbian documentation: <https://docs.armbian.com>
