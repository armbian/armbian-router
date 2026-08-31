<h2 align="center">
  <a href=#><img src="https://raw.githubusercontent.com/armbian/.github/master/profile/logosmall.png" alt="Armbian logo"></a>
  <br><br>
</h2>

# Armbian Redirector

## Purpose of This Repository

This repository contains the source code for the **Armbian redirector service** (`dlrouter`), which performs intelligent redirection for Armbian OS image downloads and APT package archive access. It routes clients to an optimal mirror based on GeoIP proximity, weights, health checks, and configurable rules.

## Features

- Written in **Go** (module `github.com/armbian/redirector`, Go 1.21).
- GeoIP + distance-based routing using MaxMind (`maxminddb-golang`) with a MMDB database.
- Weighted server pooling — the top N candidates are considered rather than a single mirror.
- Health checks for **HTTP** and **TLS** endpoints.
- Optional path remapping via a downloadable mapping file (symlink-like behavior).
- Prometheus metrics endpoint.
- HTTP router built on `go-chi/chi` with `logrus` logging middleware.
- Configuration via `viper` (YAML).
- Test suite using **Ginkgo v2** and **Gomega**.

## Checks

### HTTP

Verifies server accessibility via HTTP. If the server returns a forced redirect to an `https://` URL, it is considered HTTPS-only. If the server responds on the `https` URL with a forced `http` redirect, it is marked down due to misconfiguration — requests should never downgrade.

### TLS

Certificate checking to ensure no servers are used with invalid or expired certificates. The check loads the Mozilla CA certificate list (via `gwatts/rootcerts`) on start/reload to verify roots.

Note: The CA bundle is fetched at startup/reload; this depends on Mozilla's repository availability.

## Configuration

### Modes

#### Redirect
Standard redirect functionality.

#### Download Mapping
Uses the `dl_map` configuration variable to enable mapping of paths to new paths. Think symlinks, but in a generated file.

### Mirrors
Mirror targets (with trailing slash) are defined in the YAML configuration file.

### Example YAML

```yaml
# GeoIP Database Path
geodb: GeoLite2-City.mmdb

# Comment out to disable
dl_map: userdata.csv

# LRU Cache Size (in items)
cacheSize: 1024

# Server definition
# Weights are just like nginx, where if it's > 1 it'll be chosen x out of x + total times.
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
  # Example of a server with additional protocols (rsync).
  # Lets us potentially add an endpoint to say "give me a server with rsync".
  - server: mirrors.dotsrc.org/armbian-apt/
    weight: 15
    protocols:
      - http
      - https
      - rsync
  # Example of a server with rules.
  - server: armbian.lv.auroradev.org/apt/
    rules:
      # Required: field
      # Value matchers: is, is_not, in, not_in
      # This example excludes Google's ASN from this mirror.
      - field: asn.autonomous_system_number
        is_not: 15169
      # An example of a country blocking access to another.
      - field: location.country.iso_code
        not_in:
          - RU
```

A sample configuration is included as `dlrouter.yaml`.

## API

| Endpoint | Description |
| --- | --- |
| `/status` | Simple health check (usable by nginx/other frontends). |
| `/reload` | Flushes cache and reloads configuration and mapping. Requires `reloadToken` in config and matching `Authorization: Bearer TOKEN`. |
| `/mirrors` | All mirrors in the legacy (by region) format. |
| `/mirrors.json` | All mirrors in JSON format (see example below). |
| `/mirrors/{server}.svg` | Dynamic SVG status badge for a given server. |
| `/dl_map` | JSON-encoded download mappings. |
| `/geoip` | GeoIP information for the requester. |
| `/region/{REGIONCODE}/{PATH}` | Redirects into a specific region: `NA` (North America), `EU` (Europe), `AS` (Asia). |
| `/metrics` | Prometheus metrics endpoint (public). |

Example `/mirrors.json` payload:

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

Status badge assets used by `/mirrors/{server}.svg` live under `assets/` (`status-up.svg`, `status-down.svg`, `status-unknown.svg`).

## Repository Layout

```
cmd/main.go              Application entry point
cmd/db/genaccessors.go   Code generator for db accessors
db/                      GeoIP data structures and generated accessors
middleware/              HTTP middleware
util/                    Certificate loading and misc helpers
assets/                  SVG status badges
check.go / check_test.go Health check logic and tests
config.go                Configuration loading (viper)
http.go                  HTTP routes and handlers
map.go / map_test.go     Download mapping
mirrors.go               Mirror list handling
redirector.go            Core redirector logic
servers.go               Server pool / selection
dlrouter.yaml            Example configuration
Dockerfile               Container build definition
```

## Building

Locally with Go:

```sh
go build -o dlrouter ./cmd/main.go
```

With Docker (multi-stage build producing a distroless image):

```sh
docker build -t armbian-router .
docker run --rm -v $(pwd)/dlrouter.yaml:/dlrouter.yaml armbian-router
```

Prebuilt container images are published to GHCR at `ghcr.io/armbian/armbian-router`.

## Testing

Tests are written with Ginkgo v2 / Gomega. To run them:

```sh
go install github.com/onsi/ginkgo/v2/ginkgo@latest
ginkgo --randomize-all --p --cover --coverprofile=cover.out .
go tool cover -func=cover.out
```

See `check_test.go` and `map_test.go` for examples.

## Contributing

Contributions are welcome — bug fixes, tests, and cleanup are especially appreciated. See `check_test.go` for example test style.

## Continuous Integration

CI status and workflow history for this repository are available on the Armbian CI dashboard:

<https://actions.armbian.com/?repo=armbian-router>

## License

Distributed under the ISC-style license — see [`LICENSE`](LICENSE). Copyright (c) 2022 Tyler Stuyfzand and the Armbian Project.

## Related Links

- Armbian project: <https://www.armbian.com>
- Documentation: <https://docs.armbian.com>
