<h2 align="center">
  <a href=#><img src="https://raw.githubusercontent.com/armbian/.github/master/profile/logosmall.png" alt="Armbian logo"></a>
  <br><br>
</h2>

# Armbian Router (dlrouter)

## Purpose of This Repository

This repository contains the source code for the **Armbian redirector service** (`dlrouter`), which routes users to the optimal mirror when downloading Armbian OS images and accessing the APT package archive. It acts as a central, GeoIP-aware entry point for Armbian's distributed mirror network.

## Features

- GeoIP + distance-based routing
- Weighted server pooling (the top N nearest/eligible servers are considered rather than a single one)
- Health checks over HTTP and TLS
- Optional download path mapping (symlink-like remapping via CSV)
- LRU caching of routing decisions
- Prometheus metrics endpoint
- SVG status badges for dynamic mirror lists

## Built With

- **Go** (module `github.com/armbian/redirector`, `go 1.21`)
- HTTP routing via [`go-chi/chi`](https://github.com/go-chi/chi) with a logrus logger middleware
- Configuration via [`spf13/viper`](https://github.com/spf13/viper) (YAML)
- GeoIP lookups via [`oschwald/maxminddb-golang`](https://github.com/oschwald/maxminddb-golang)
- Root CA bundle via [`gwatts/rootcerts`](https://github.com/gwatts/rootcerts) (Mozilla CA list)
- Weighted random selection via [`jmcvetta/randutil`](https://github.com/jmcvetta/randutil)
- LRU cache via [`hashicorp/golang-lru`](https://github.com/hashicorp/golang-lru)
- Concurrency helpers via [`sourcegraph/conc`](https://github.com/sourcegraph/conc) and [`samber/lo`](https://github.com/samber/lo)
- Metrics via [`prometheus/client_golang`](https://github.com/prometheus/client_golang)
- Logging via [`sirupsen/logrus`](https://github.com/sirupsen/logrus)
- Testing with [Ginkgo v2](https://github.com/onsi/ginkgo) and [Gomega](https://github.com/onsi/gomega)
- Container image built from `golang:alpine` onto `gcr.io/distroless/static:nonroot`

## Repository Layout

```
.
├── cmd/
│   ├── main.go            # Service entry point (built as `dlrouter`)
│   └── db/genaccessors.go # Code generator for db accessors
├── db/
│   ├── accessors.go       # Generated GeoIP field accessors
│   └── structs.go
├── middleware/
│   └── middleware.go
├── util/
│   ├── certificates.go    # Mozilla CA bundle loading / TLS helpers
│   └── util.go
├── assets/
│   ├── status-up.svg
│   ├── status-down.svg
│   └── status-unknown.svg
├── check.go / check_test.go
├── config.go
├── http.go
├── map.go / map_test.go
├── mirrors.go
├── redirector.go
├── servers.go
├── armbianmirror_suite_test.go
├── dlrouter.yaml          # Example configuration
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

A multi-stage `Dockerfile` produces a minimal distroless image:

```sh
docker build -t armbian-router .
docker run --rm -v $(pwd)/dlrouter.yaml:/dlrouter.yaml armbian-router
```

Prebuilt images are published to GitHub Container Registry:

```
ghcr.io/armbian/armbian-router:latest
```

## Testing

Tests use Ginkgo v2:

```sh
go install github.com/onsi/ginkgo/v2/ginkgo@latest
ginkgo --randomize-all --p --cover --coverprofile=cover.out .
go tool cover -func=cover.out
```

See `check_test.go` for example tests.

## Checks

The supported health checks are HTTP and TLS.

### HTTP

Verifies server accessibility via HTTP. If the server returns a forced redirect to an `https://` URL, it is considered HTTPS-only.

If the server responds on the `https` URL with a forced `http` redirect, it will be marked down due to misconfiguration. Requests should never downgrade.

### TLS

Certificate checking to ensure no servers are used which have invalid/expired certificates. This check uses the Mozilla CA certificate list (via `gwatts/rootcerts`), loaded on start/config reload, to verify roots.

OS certificate trusts were previously used, however issues with date validation prompted the move to the bundled Mozilla CA list, which is more portable and predictable.

Note: The CA bundle is refreshed from GitHub on every startup/reload.

## Configuration

Configuration is read from a YAML file (see `dlrouter.yaml`).

### Modes

#### Redirect

Standard redirect functionality.

#### Download Mapping

Uses the `dl_map` configuration variable to enable mapping of paths to new paths — think symlinks, but expressed in a generated CSV file.

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
      # field: any field exposed by the GeoIP accessors
      # Value matchers: is, is_not, in, not_in
      - field: asn.autonomous_system_number
        is_not: 15169
      - field: location.country.iso_code
        not_in:
          - RU
```

## API

| Path | Description |
|---|---|
| `/status` | Simple health check (suitable for nginx `502`-style probes). |
| `/reload` | Flushes cache and reloads configuration and mapping. Requires `reloadToken` in config and a matching `Authorization: Bearer TOKEN` header. |
| `/mirrors` | All mirrors in the legacy (by region) format. |
| `/mirrors.json` | All mirrors in the new JSON format. |
| `/mirrors/{server}.svg` | SVG status badge for a given server, for dynamic mirror lists. |
| `/dl_map` | JSON-encoded download mappings. |
| `/geoip` | GeoIP information for the requester. |
| `/region/{REGIONCODE}/{PATH}` | Redirect to the desired region (`NA` – North America, `EU` – Europe, `AS` – Asia). |
| `/metrics` | Prometheus metrics endpoint (public). |

Example `/mirrors.json` output:

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

For an overview of the CI workflows in this repository and their current status, see the Armbian CI dashboard:

<https://actions.armbian.com/?repo=armbian-router>

## Contributing

Contributions are welcome. Work is ongoing to standardize the code, improve tests, and clean up rough edges — see `check_test.go` for example test patterns and open a PR.

## More Information

- Armbian website: <https://www.armbian.com>
- Armbian documentation: <https://docs.armbian.com>

## License

Distributed under an ISC-style license. See [`LICENSE`](LICENSE) for details.  
Copyright (c) 2022 Tyler Stuyfzand and the Armbian Project.
