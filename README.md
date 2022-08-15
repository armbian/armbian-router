Armbian Redirector
==================

This repository contains a redirect service for Armbian downloads, apt, etc.

It uses multiple current technologies and best practices, including:

- Go 1.19
- GeoIP + Distance routing
- Server weighting, pooling (top x servers are served instead of a single one)
- Health checks (HTTP, TLS)

Code Quality
------------

The code quality isn't the greatest/top tier. All code lives in the "main" package and should be moved at some point.

Regardless, it is meant to be simple and easy to understand.

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
  - server: mirrors.dotsrc.org/armbian-apt/
    weight: 15
    protocols:
      - rsync
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