<h2 align="center">
  <img src="https://raw.githubusercontent.com/armbian/.github/master/profile/logo.png" alt="Armbian logo" width="25%">
  <br><br>
</h2>

### Purpose of This Repository

This repository contains the **source code for the Armbian redirector service**, which handles intelligent redirection for Armbian OS image downloads and APT package archive access. The redirector ensures that users are routed to the optimal mirror or resource location based on availability, geographic proximity, or request type. It acts as a central entry point for distributed Armbian services.

It uses multiple current technologies and best practices, including:

- Go 1.19
- Ginkgo v2 and Gomega testing framework
- GeoIP + Distance routing
- Server weighting, pooling (top x servers are served instead of a single one)
- Health checks (HTTP, TLS)

## Code Quality

The code quality isn't the greatest/top tier. Work is being done towards cleaning it up and standardizing it, writing tests, etc.

All contributions are welcome, see the `check_test.go` file for example tests.

## Checks

The supported checks are HTTP and TLS.

### HTTP

Verifies server accessibility via HTTP. If the server returns a forced redirect to an `https://` url, it is considered to be https-only.

If the server responds on the `https` url with a forced `http` redirect, it will be marked down due to misconfiguration. Requests should never downgrade.

### TLS

Certificate checking to ensure no servers are used which have invalid/expired certificates. This check is written to use the Mozilla ca certificate list, loaded on start/config load, to verify roots.

OS certificate trusts WERE being used to do this, however some issues with the date validation (which could be user error) caused the move to the ca bundle, which could be considered more usable.

Note: This downloads from github every startup/reload. This should be a reliable process, as long as Mozilla doesn't deprecate their repo. Their HG URL is super slow.

Configuration
-------------

### Modes

#### Redirect

Standard redirect functionality

#### Download Mapping

Uses the `dl_map` configuration variable to enable mapping of paths to new paths.

Think symlinks, but in a generated file.

### Mirrors
Mirror targets with trailing slash are placed in the yaml configuration file.

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
  # Useful for defining servers which could be used for rsync sources
  # This lets us potentially add an endpoint to say "give me a server with rsync"
  - server: mirrors.dotsrc.org/armbian-apt/
    weight: 15
    protocols:
      - http
      - https
      - rsync
  # Example of a server with rules
  - server: armbian.lv.auroradev.org/apt/
    rules:
      # Required: field
      # Value matchers: is, is_not, in, not_in
      # See the RuleInput struct, as well as the ASN and 
      # This example excludes Google's ASN from this mirror
      - field: asn.autonomous_system_number
        is_not: 15169
      # An example of a country blocking access to another
      # For instance, Ukraine not allowing Russian ISPs in.
      - field: location.country.iso_code
        not_in:
          - RU
````

## API

`/status`

Meant for a simple health check (nginx/etc can 502 or similar if down)

`/reload`

Flushes cache and reloads configuration and mapping. Requires reloadToken to be set in the configuration, and a matching token provided in `Authorization: Bearer TOKEN`

`/mirrors`

Shows all mirrors in the legacy (by region) format

`/mirrors.json`

Shows all mirrors in the new JSON format. Example:

```json
[
  {
    "available":true,
    "host":"imola.armbian.com",
    "path":"/apt/",
    "latitude":46.0503,
    "longitude":14.5046,
    "weight":10,
    "continent":"EU",
    "lastChange":"2022-08-12T06:52:35.029565986Z"
  }
]
```

`/mirrors/{server}.svg`

Magic SVG path to show badges based on server status, for use in dynamic mirror lists.

`/dl_map`

Shows json-encoded download mappings

`/geoip`

Shows GeoIP information for the requester

`/region/REGIONCODE/PATH`

Using this magic path will redirect to the desired region:

* NA - North America
* EU - Europe
* AS - Asia

`/metrics`

Prometheus metrics endpoint. Metrics aren't considered private, thus are exposed to the public.
