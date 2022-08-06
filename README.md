Armbian Redirector
==================

This repository contains a redirect service for Armbian downloads, apt, etc.

It uses multiple current technologies and best practices, including:

- Go 1.17/1.18
- GeoIP + Distance routing
- Server weighting, pooling (top x servers are served instead of a single one)
- Health checks (HTTP, TLS)

Code Quality
------------

The code quality isn't the greatest/top tier. All code lives in the "main" package and should be moved at some point.

Regardless, it is meant to be simple and easy to understand.