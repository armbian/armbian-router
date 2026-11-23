<h2 align="center">
  <a href=#><img src="https://raw.githubusercontent.com/armbian/.github/master/profile/logosmall.png" alt="Armbian logo"></a>
  <br><br>
</h2>

# Armbian Router (dlrouter)

## Purpose of This Repository

This repository contains the source code for the **Armbian redirector service** (`dlrouter`), an intelligent HTTP router that redirects users to the best available Armbian mirror for OS images and APT package archives based on geographic proximity, server weight, health status, and optional per-mirror rules.

## Features

- Written in **Go** (module `github.com/armbian/redirector`, Go 1.21)
- GeoIP + distance-based routing (via MaxMind GeoLite2)
- Server weighting and pooling (the top N servers are considered, not just one)
- Health checks (HTTP and TLS/certificate validation)
- LRU-cached redirect decisions
- Optional path-to-path download mapping (`dl_map`)
- Prometheus metrics endpoint
- Dynamic SVG status badges for mirrors
- Tests using [Ginkgo v2](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/)

## Build

### Local build

```sh
go build -o dlrouter ./cmd/main.go
```

### Docker

A multi-stage `Dockerfile` is provided that produces a minimal distroless image:

```sh
docker build -t armbian-router .
docker run --rm -v "$PWD/dlrouter.yaml:/dlrouter.yaml" armbian-router
```

Prebuilt images are published to GitHub Container Registry at `ghcr.io/armbian/armbian-router`.

## Checks

The supported health checks are **HTTP** and **TLS**.

### HTTP

Verifies server accessibility via HTTP. If the server returns a forced redirect to an `https://` URL, it is considered HTTPS-only.

If the server responds on the `https` URL with a forced `http` redirect, it will be marked down due to misconfiguration — requests should never downgrade.

### TLS

Certificate checking ensures that no servers with invalid or expired certificates are used. The check loads the Mozilla CA certificate list on startup/reload to verify roots.

OS certificate trusts were previously used, however issues with date validation (possibly user error) prompted the move to the CA bundle, which is considered more portable. The bundle is fetched from GitHub on every startup/reload.

## Configuration

Configuration is loaded via [Viper](https://github.com/spf13/viper). See `dlrouter.yaml` for a working example.

### Modes

#### Redirect

Standard redirect functionality.

#### Download Mapping

Enabled via the `dl_map` configuration variable. Maps request paths to new paths — think of it as symlinks expressed in a generated file.

### Mirrors

Mirror targets (with trailing slash) are declared in the YAML configuration file.

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
      # This example excludes Google's ASN from this mirror
      - field: asn.autonomous_system_number
        is_not: 15169
      # An example of a country blocking access to another
      - field: location.country.iso_code
        not_in:
          - RU
```

## HTTP API

| Endpoint | Description |
| --- | --- |
| `/status` | Simple health check endpoint (suitable for nginx upstream probing). |
| `/reload` | Flushes cache and reloads configuration/mapping. Requires `reloadToken` in config and a matching `Authorization: Bearer TOKEN` header. |
| `/mirrors` | All mirrors in the legacy (grouped-by-region) format. |
| `/mirrors.json` | All mirrors in the new JSON format. |
| `/mirrors/{server}.svg` | SVG status badge for a given server, for dynamic mirror lists. |
| `/dl_map` | JSON-encoded download mappings. |
| `/geoip` | GeoIP information for the requester. |
| `/region/{REGIONCODE}/{PATH}` | Redirects to the desired region (`NA`, `EU`, `AS`). |
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

## Repository Layout

```
cmd/           # Program entry point(s); main.go builds the dlrouter binary
db/            # GeoIP data structures and generated accessors
middleware/    # HTTP middleware
util/          # Helpers (certificate loading, misc utilities)
assets/        # SVG status badges (up / down / unknown)
*.go           # Core redirector: check, config, http, map, mirrors, servers, ...
Dockerfile     # Multi-stage build producing a distroless image
dlrouter.yaml  # Example configuration
```

## Testing

Tests are written with Ginkgo v2 / Gomega. To run them:

```sh
go install github.com/onsi/ginkgo/v2/ginkgo@latest
ginkgo --randomize-all --p --cover --coverprofile=cover.out .
go tool cover -func=cover.out
```

See `check_test.go` and `map_test.go` for examples.

## Contributing

The code quality isn't top tier — work is ongoing to clean it up, standardize it, and expand test coverage. All contributions are welcome. See `check_test.go` for an example of how tests are structured.

## Continuous Integration

An overview of this repository's CI pipelines and their current status is available on the Armbian CI dashboard:

<https://actions.armbian.com/?repo=armbian-router>

## License

Released under the ISC license. See [`LICENSE`](LICENSE).

Copyright © 2022 Tyler Stuyfzand &lt;admin@meow.tf&gt;, Armbian Project.

## Related Links

- Armbian website: <https://www.armbian.com>
- Armbian documentation: <https://docs.armbian.com>
