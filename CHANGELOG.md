# Changelog

This changelog contains the changes made between releases. The versioning follows [Semantic Versioning](https://semver.org/).

## Unreleased

## [3.0.0+fork]

### Added

- **HomeCoach Support**: Full integration of Netatmo HomeCoach devices
  - Renamed old metrics endpoint `/metrics/v1` with separate collectors for Weather and HomeCoach
    - Maintained backward compatibility for existing dashboards reliant on Weather Metrics
  - New unified metrics endpoint `/metrics/v2` with consistent label schema across weather and homecoach devices
  - HomeCoach-specific metrics: temperature, humidity, CO2, noise, pressure, health index, WiFi signal
- **Feature Flags**: Granular control over enabled collectors
  - `--enable-weather` / `NETATMO_ENABLE_WEATHER`: Enable/disable Weather station collector (default: true)
  - `--enable-homecoach` / `NETATMO_ENABLE_HOMECOACH`: Enable/disable HomeCoach collector (default: true)
  - `--enable-go-metrics` / `NETATMO_ENABLE_GO_METRICS`: Enable/disable Go runtime metrics (default: false)
- **Enhanced Debug Handler**: Combined debug endpoint showing both Weather and HomeCoach data
  - Accessible at `/debug/netatmo` when debug handlers are enabled
  - Returns JSON with separate sections for weather and homecoach data

### Changed

- **Refactored Collector Architecture**: Split monolithic collector into modular components
  - `internal/collector/weather.go`: Weather station specific collector
  - `internal/collector/homecoach.go`: HomeCoach specific collector
  - `internal/collector/common.go`: Shared V2 unified collector and helper functions
- **Dual Registry System**: Separate Prometheus registries for V1 and V2 metrics
  - V1 registry maintains backward compatibility with existing dashboards
  - V2 registry provides unified metrics schema for new deployments
- **Go Runtime Metrics**: Now disabled by default to reduce metric noise
  - Can be enabled via `--enable-go-metrics` flag when needed for debugging

## [2.5.0+fork] - 2025-11-15

### Added

- Delete token functionality in web interface
- Added Homecoach Scope to oauth authorization process based on enabled collectors

## [2.4.3+fork] - 2025-11-15

### Fixed

- Debug handler `/debug/netatmo` now correctly handles disabled collectors (Weather/HomeCoach)

## [2.4.2+fork] - 2025-11-13

### Changed

- Improved code consistency across collector files
- Code formatting and style improvements

## [2.4.1+fork] - 2025-11-12

### Changed

- Small fixes and preparation for new V2 endpoint
- Internal refactoring for upcoming unified metrics API

## [2.4.0+fork] - 2025-11-11

### Added
- Docker usage instructions in README
- Environment variable configuration documentation
- WiFi signal strength metric to HomeCoach collector
- Combined debug handler for Weather and HomeCoach data at `/debug/netatmo`

### Changed

- Updated GitHub workflows: removed old workflows, added new Docker image build and push workflow
- Enhanced HomeCoach data structure
- Updated maintainer information and module path

## [2.3.0+fork] - 2025-11-10

### Added

- Support to disable Go runtime metrics in endpoint
- Configuration option `--enable-go-metrics` / `NETATMO_ENABLE_GO_METRICS`

## [2.2.0+fork] - 2025-11-09

### Added

- Support for Netatmo Homecoach devices
- HomeCoach-specific status metrics to Prometheus monitoring
- Initial HomeCoach collector implementation with metrics endpoint

## [2.1.2] - 2025-08-21

### Changed

- Updated API endpoint to `api.netatmo.com`
- Updated Go runtime and dependencies

## [2.1.1] - 2024-12-22

### Added

- Error messages returned from NetAtmo API are shown in more detail

### Changed

- Updated Go runtime and dependencies

## [2.1.0] - 2024-10-20

### Added

- Show version information on startup
- Token is saved during runtime of exporter once it is refreshed and not just on shutdown

### Fixed

- Ignore expired tokens on startup

### Changed

- Updated Go runtime and dependencies

## [2.0.1] - 2023-10-15

### Changed

- Maintenance release, updates Go runtime and dependencies

## [2.0.0] - 2023-07-18

- Major: New authentication method replaces existing username/password authentication

## [1.5.1] - 2023-01-08

### Changed

- `latest` Docker tag now points to most recent release and `master` points to the build from the default branch

## [1.5.0] - 2022-12-06

### Added

- Debugging endpoint for looking at data read from NetAtmo API (`/debug/data`)
- New `home` label as additional identification for sensors
- Use module ID (currently MAC-address) as fallback for the `name` label if no name is provided

### Changed

- Switch to fork of netatmo-api-go library

### Fixed

- Not all metric descriptors sent to registry

## [1.4.0] - 2022-04-02

### Changed

- Go 1.17

### Fixed

- Updated Prometheus client library for CVE-2022-21698
- lastRefreshError is not reset (#11)
- Docker build for arm64

## [1.3.0] - 2020-08-09

### Added

- HTTP Handler for getting build information `/version`
- In-memory cache for data retrieved from NetAtmo API, configurable timeouts

### Changed

- Logger uses leveled logging, added option to set log level
- Updated Go runtime and dependencies

## [1.2.0] - 2018-10-27

### Added

- Support for battery and RF-link status
- Support for configuration via environment variables

## [1.1.0] - 2018-09-02

### Added

- Support for wind and rain sensors

### Changed

- Metrics now also contain a label for the "station name"

## [1.0.1] - 2017-11-26

### Fixed

- Integrate fix of upstream library

## [1.0.0] - 2017-03-09

- Initial release

[2.1.2]: https://github.com/xperimental/netatmo-exporter/releases/tag/v2.1.2
[2.1.1]: https://github.com/xperimental/netatmo-exporter/releases/tag/v2.1.1
[2.1.0]: https://github.com/xperimental/netatmo-exporter/releases/tag/v2.1.0
[2.0.1]: https://github.com/xperimental/netatmo-exporter/releases/tag/v2.0.1
[2.0.0]: https://github.com/xperimental/netatmo-exporter/releases/tag/v2.0.0
[1.5.1]: https://github.com/xperimental/netatmo-exporter/releases/tag/v1.5.1
[1.5.0]: https://github.com/xperimental/netatmo-exporter/releases/tag/v1.5.0
[1.4.0]: https://github.com/xperimental/netatmo-exporter/releases/tag/v1.4.0
[1.3.0]: https://github.com/xperimental/netatmo-exporter/releases/tag/v1.3.0
[1.2.0]: https://github.com/xperimental/netatmo-exporter/releases/tag/v1.2.0
[1.1.0]: https://github.com/xperimental/netatmo-exporter/releases/tag/v1.1.0
[1.0.1]: https://github.com/xperimental/netatmo-exporter/releases/tag/v1.0.1
[1.0.0]: https://github.com/xperimental/netatmo-exporter/releases/tag/v1.0.0
