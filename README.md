<h2 align="center">
  <a href=#><img src="https://raw.githubusercontent.com/armbian/.github/master/profile/logosmall.png" alt="Armbian logo"></a>
  <br><br>
</h2>

# Armbian Router (dlrouter)

## Purpose of This Repository

This repository contains the source code for the **Armbian redirector service**, which routes clients to the optimal mirror for Armbian OS image downloads and APT package archive access. It makes routing decisions based on availability, geographic proximity, server weighting, and configurable rules.

## Features

- Written in **Go** (module `github.com/armbian/redirector`, Go 1.21).
- GeoIP + distance-based routing using the MaxMind GeoLite2 database.
- Server weighting and pooling — the top N candidate servers are considered rather than a single fixed pick.
- Health checks over HTTP and TLS, with TLS verification against the Mozilla CA bundle (via `github.com/gwatts/rootcerts`).
- LRU caching for routing decisions.
- Prometheus metrics endpoint.
- Download path mapping (symlink-like redirects via a generated map).
- Tested with **Ginkgo v2** / **Gomega**.
- Packaged as a minimal **distroless** Docker image.

## Repository Layout

```
.
├── cmd/
│   ├── main.go              # Application entry point
│   └── db/genaccessors.go   # Code generator for db accessors
├── db/
│   ├── accessors.go         # Generated/handwritten GeoIP record accessors
│   └── structs.go
├── middleware/middleware.go
├── util/
│   ├── certificates.go      # TLS/CA bundle helpers
│   └── util.go
├── assets/                  # status-up/down/unknown SVG badges
├── check.go / check_test.go # HTTP & TLS health checks
├── config.go                # Configuration loading (Viper)
├── http.go                  # HTTP routes/handlers
├── map.go / map_test.go     # Download path mapping
├── mirrors.go               # Mirror model and listings
├── redirector.go            # Core redirection logic
├── servers.go               # Server selection / pooling
├── armbianmirror_suite_test.go
├── dlrouter.yaml            # Example configuration
├── Dockerfile
├── go.mod / go.sum
└── .github/workflows/       # CI pipelines
```

## Building

### From source

Requires Go 1.21+.

```sh
go build -o dlrouter ./cmd/main.go
```

### Docker

A multi-stage `Dockerfile` builds a static binary and ships it on a distroless base image. Pre-built images are published to GitHub Container Registry:

```sh
docker pull ghcr.io/armbian/armbian-router:latest
docker run --rm -v $PWD/dlrouter.yaml:/dlrouter.yaml ghcr.io/armbian/armbian-router:latest
```

## Testing

Tests use the [Ginkgo v2](https://onsi.github.io/ginkgo/) framework:

```sh
go install github.com/onsi/ginkgo/v2/ginkgo@latest
ginkgo --randomize-all --p --cover --coverprofile=cover.out .
go tool cover -func=cover.out
```

See `check_test.go` for example tests. Contributions that add test coverage are especially welcome.

## Checks

The supported health checks are HTTP and TLS.

### HTTP

Verifies server accessibility via HTTP. If the server returns a forced redirect to an `https://` URL, it is considered HTTPS-only.

If a server responds on the `https` URL with a forced `http` redirect, it is marked down due to misconfiguration — requests should never downgrade.

### TLS

Certificate validation ensures no mirrors with invalid/expired certificates are used. The check loads the Mozilla CA bundle on start/config reload to verify roots.

The OS trust store was previously used, however date-validation issues (possibly user error) prompted the move to the bundled Mozilla CA list, which is more portable. The bundle is fetched from GitHub on each startup/reload.

## Configuration

### Modes

#### Redirect

Standard redirect functionality.

#### Download Mapping

Uses the `dl_map` configuration variable to enable mapping of paths to new paths — think symlinks, but in a generated file.

### Mirrors

Mirror targets (with trailing slash) are placed in the YAML configuration file.

### Example YAML

```yaml
# GeoIP Database Path
geodb: GeoLite2-City.mmdb

# Comment out to disable
dl_map: userdata.csv

# LRU Cache Size (in items)
cacheSize: 1024

# Server definition
# Weights work like nginx: weight > 1 increases selection probability.
# By default, the top 3 servers are considered when choosing the best.
# server    = full URL or host+path
# weight    = int
# optional: latitude, longitude (float)
# optional: protocols (list)
servers:
  - server: armbian.12z.eu/apt/
  - server: armbian.chi.auroradev.org/apt/
    weight: 15
    latitude: 41.8879
    longitude: -88.1995
  # Server advertising additional protocols (e.g. rsync).
  - server: mirrors.dotsrc.org/armbian-apt/
    weight: 15
    protocols:
      - http
      - https
      - rsync
  # Server with routing rules.
  - server: armbian.lv.auroradev.org/apt/
    rules:
      # Required: field
      # Value matchers: is, is_not, in, not_in
      # Exclude Google's ASN from this mirror.
      - field: asn.autonomous_system_number
        is_not: 15169
      # Example: block a country from using this mirror.
      - field: location.country.iso_code
        not_in:
          - RU
```

## API

| Path | Description |
| --- | --- |
| `/status` | Simple health check (useful for upstream nginx-style failover). |
| `/reload` | Flush cache and reload configuration/mapping. Requires `reloadToken` in config and a matching `Authorization: Bearer TOKEN` header. |
| `/mirrors` | All mirrors in the legacy (by region) format. |
| `/mirrors.json` | All mirrors in the new JSON format. |
| `/mirrors/{server}.svg` | SVG badge reflecting the server's current status. |
| `/dl_map` | JSON-encoded download mappings. |
| `/geoip` | GeoIP information for the requester. |
| `/region/{REGIONCODE}/{PATH}` | Redirect to a mirror in the given region (`NA`, `EU`, `AS`). |
| `/metrics` | Prometheus metrics endpoint (public). |

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

CI runs via GitHub Actions (build/test) and a workflow that publishes Docker images to GHCR on tag pushes. For a live overview of this repository's pipelines, see:

<https://actions.armbian.com/?repo=armbian-router>

## Code Quality

The code is a work-in-progress — cleanup, standardization, and more test coverage are ongoing. Contributions are welcome; see `check_test.go` for examples of how tests are structured.

## License

Distributed under the ISC License. See [`LICENSE`](LICENSE) for details.

Copyright © 2022 Tyler Stuyfzand and the Armbian Project.

## Related

- Armbian: <https://www.armbian.com>
- Documentation: <https://docs.armbian.com>
