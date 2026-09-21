<h2 align="center">
  <a href=#><img src="https://raw.githubusercontent.com/armbian/.github/master/profile/logosmall.png" alt="Armbian logo"></a>
  <br><br>
</h2>

# Armbian Router (dlrouter)

## Purpose of This Repository

This repository contains the source code for the **Armbian redirector service** (`dlrouter`), a Go application that intelligently redirects users to the optimal mirror for Armbian OS image downloads and APT package archives, using GeoIP-based routing, server weighting, and active health checks.

## Features

- **GeoIP + distance routing** using MaxMind (`GeoLite2-City.mmdb`)
- **Server weighting and pooling** — the top N candidate servers are chosen from rather than a single fixed target
- **Health checks** over HTTP and TLS
- **Download path mapping** — symlink-like remapping via a generated CSV file
- **Prometheus metrics** endpoint
- **Per-server rules** to include/exclude clients based on ASN, country, etc.
- **SVG status badges** for dynamic mirror lists

## Built With

- **Go 1.21** (module `github.com/armbian/redirector`; see `go.mod`)
- HTTP routing via [`go-chi/chi`](https://github.com/go-chi/chi) with `chi-middleware/logrus-logger`
- Configuration via [`spf13/viper`](https://github.com/spf13/viper) (YAML)
- GeoIP lookups via [`oschwald/maxminddb-golang`](https://github.com/oschwald/maxminddb-golang)
- Root certificate bundle via [`gwatts/rootcerts`](https://github.com/gwatts/rootcerts) (Mozilla CA list)
- Metrics via [`prometheus/client_golang`](https://github.com/prometheus/client_golang)
- Logging via [`sirupsen/logrus`](https://github.com/sirupsen/logrus)
- Testing via [Ginkgo v2](https://github.com/onsi/ginkgo) and [Gomega](https://github.com/onsi/gomega)
- Containerization via a multi-stage Docker build on `distroless/static:nonroot`

## Repository Layout

```
.
├── cmd/
│   ├── main.go            # dlrouter entry point
│   └── db/genaccessors.go # code generator for db accessors
├── db/
│   ├── accessors.go       # (generated) field accessors used by rule matching
│   └── structs.go
├── middleware/
│   └── middleware.go
├── util/
│   ├── certificates.go    # Mozilla CA bundle loading for TLS checks
│   └── util.go
├── assets/                # status-up/down/unknown SVG badges
├── check.go               # HTTP / TLS health checks
├── config.go              # Viper-based configuration
├── http.go                # HTTP handlers (API endpoints)
├── map.go                 # download path mapping
├── mirrors.go             # mirror list handling
├── redirector.go          # core redirect logic
├── servers.go             # server pool / weighting
├── dlrouter.yaml          # example configuration
├── Dockerfile
└── go.mod / go.sum
```

## Building

### From source

```sh
go build -o dlrouter ./cmd/main.go
```

### With Docker

```sh
docker build -t armbian-router .
docker run --rm -v $PWD/dlrouter.yaml:/dlrouter.yaml armbian-router
```

Prebuilt images are published to `ghcr.io/armbian/armbian-router:latest`.

## Testing

Tests are written with Ginkgo/Gomega. To run them locally:

```sh
go install github.com/onsi/ginkgo/v2/ginkgo@latest
ginkgo --randomize-all --p --cover --coverprofile=cover.out .
go tool cover -func=cover.out
```

See `check_test.go` and `map_test.go` for examples. Contributions of additional tests are welcome.

## Checks

The supported checks are HTTP and TLS.

### HTTP

Verifies server accessibility via HTTP. If the server returns a forced redirect to an `https://` URL, it is considered HTTPS-only.

If the server responds on the `https` URL with a forced `http` redirect, it will be marked down due to misconfiguration. Requests should never downgrade.

### TLS

Certificate checking to ensure no servers are used which have invalid/expired certificates. This check uses the Mozilla CA certificate list, loaded on start/config reload, to verify roots.

OS certificate trusts were previously used, but some issues with date validation (which could be user error) motivated the move to the CA bundle, which is considered more portable.

Note: The bundle is fetched from GitHub on every startup/reload. This should be reliable as long as Mozilla doesn't deprecate the source repo.

## Configuration

### Modes

**Redirect** — standard redirect functionality.

**Download Mapping** — uses the `dl_map` configuration variable to enable mapping of paths to new paths, similar to symlinks but expressed in a generated file.

### Mirrors

Mirror targets (with trailing slash) are placed in the YAML configuration file. An example `dlrouter.yaml` is included in the repository.

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
  # Server exposing additional protocols such as rsync
  - server: mirrors.dotsrc.org/armbian-apt/
    weight: 15
    protocols:
      - http
      - https
      - rsync
  # Server with rules — this one excludes Google's ASN
  - server: armbian.lv.auroradev.org/apt/
    rules:
      - field: asn.autonomous_system_number
        is_not: 15169
      - field: location.country.iso_code
        not_in:
          - RU
```

Rule matchers include `is`, `is_not`, `in`, and `not_in`, operating on fields exposed by `db/accessors.go`.

## HTTP API

| Endpoint | Description |
| --- | --- |
| `/status` | Simple health check (nginx or similar can 502 if the service is down). |
| `/reload` | Flushes cache and reloads configuration and mapping. Requires `reloadToken` in the config and a matching `Authorization: Bearer TOKEN` header. |
| `/mirrors` | Legacy mirror list, grouped by region. |
| `/mirrors.json` | Mirror list in JSON format (see example below). |
| `/mirrors/{server}.svg` | Status badge for use in dynamic mirror lists. |
| `/dl_map` | JSON-encoded download mappings. |
| `/geoip` | GeoIP information for the requester. |
| `/region/{REGIONCODE}/{PATH}` | Force a redirect to a specific region (`NA`, `EU`, `AS`). |
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

Build, test, and container-publishing pipelines are automated via GitHub Actions. A live overview of workflow runs for this repository is available at:

<https://actions.armbian.com/?repo=armbian-router>

## Code Quality

Work is ongoing to clean up and standardize the codebase and expand test coverage. All contributions are welcome — see `check_test.go` for example tests.

## License

Distributed under the ISC-style license. See [`LICENSE`](LICENSE) for details.

Copyright © 2022 Tyler Stuyfzand and the Armbian Project.

## Related Links

- Armbian website: <https://www.armbian.com>
- Armbian documentation: <https://docs.armbian.com>
