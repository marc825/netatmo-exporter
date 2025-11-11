# netatmo-exporter

Simple [prometheus](https://prometheus.io) exporter for getting sensor values [NetAtmo](https://www.netatmo.com) sensors into prometheus.

**This is a fork of the original [netatmo-exporter](https://github.com/exzz/netatmo-exporter)**

**This is work in progress so no guarantees are given that this works as expected. Use at your own risk.**

Goal of this fork is to extend the original netatmo-exporter with additional features and optimize it for my personal use-cases. If you are looking for the original project, please visit the link above.

## Features

This fork includes all features of the original netatmo-exporter plus the following:
- Added monitoring for Netatmo HomeCoach/AirCare devices
- Enable/Disable monitoring for Weather and HomeCoach via environment variables
- Combined debug handler for Weather and HomeCoach data

## Installation

#### Docker

latest dev build marc825/netatmo-exporter:dev

Example using `docker run` and volume for token persistence:

```bash
docker run -d --name netatmo-exporter --restart unless-stopped `
  -p 127.0.0.1:9210:9210 `
  -v netatmo_data:/var/lib/netatmo-exporter `
  -e NETATMO_CLIENT_ID="<DEINE_CLIENT_ID>" `
  -e NETATMO_CLIENT_SECRET="<DEIN_CLIENT_SECRET>" `
  -e NETATMO_EXPORTER_EXTERNAL_URL="http://localhost:9210" `
  -e NETATMO_ENABLE_WEATHER="true" `
  -e NETATMO_ENABLE_HOMECOACH="true" `
  -e NETATMO_ENABLE_GO_METRICS="false" `
  -e DEBUG_HANDLERS="false" `
  marc825/netatmo-exporter:dev
```


#### Token file and Docker volume

When running the `netatmo-exporter` in Docker, it is recommended to store the token file in a "Docker volume", so that it can persist container recreation. The image is already set up to do that. The default path for the token file is `/var/lib/netatmo-exporter/netatmo-token.json` and the whole `/var/lib/netatmo-exporter/` directory is set as a volume.

This enables the user to update the used netatmo-exporter image without losing the authentication, for example using `docker compose`. It does not automatically provide the same mechanism on Kubernetes, though. For Kubernetes, you probably want a `StatefulSet`.

If you want to build the exporter for a different OS or architecture, you can specify arguments to the Makefile:

```bash
# For 64-bit ARM on Linux
GOOS=linux GOARCH=arm64 make build-binary
```

## NetAtmo client credentials

This application tries to get data from the NetAtmo API. For that to work you will need to create an application in the [NetAtmo developer console](https://dev.netatmo.com/apps/), so that you can get a Client ID and secret.

For authentication, you either need to use the integrated web-interface of the exporter or you need to use the developer console to create a token and make manually make it available for the exporter to use. See [authentication.md](/doc/authentication.md) for more details.

The exporter is able to persist the authentication token during restarts, so that no user interaction is needed when restarting the exporter, unless the token expired during the time the exporter was not active. See [token-file.md](/doc/token-file.md) for an explanation of the file used for persisting the token.

### Required Netatmo API Scopes

|                        Variable | Description                                                                |
|--------------------------------:|----------------------------------------------------------------------------|
| read_station                    | Read access to the NetAtmo weather station data.                          |
| read_homecoach                  | Read access to the NetAtmo HomeCoach data.                                |

## Usage

```plain
$ netatmo-exporter --help
Usage of netatmo-exporter:
  -a, --addr string                 Address to listen on. (default ":9210")
      --age-stale duration          Data age to consider as stale. Stale data does not create metrics anymore. (default 1h0m0s)
  -i, --client-id string            Client ID for NetAtmo app.
  -s, --client-secret string        Client secret for NetAtmo app.
      --debug-handlers              Enables debugging HTTP handlers.
      --external-url string         External URL to use as base for OAuth redirect URL.
      --log-level level             Sets the minimum level output through logging. (default info)
      --refresh-interval duration   Time interval used for internal caching of NetAtmo sensor data. (default 8m0s)
      --token-file string           Path to token file for loading/persisting authentication token.
```

After starting the server will offer the metrics on the `/metrics` endpoint, which can be used as a target for prometheus.

### Environment variables

The exporter can be configured either via command line arguments (see previous section) or by populating the following environment variables:

|                        Variable | Description                                                                |                                                   Default |
|--------------------------------:|----------------------------------------------------------------------------|----------------------------------------------------------:|
|         `NETATMO_EXPORTER_ADDR` | Address to listen on                                                       |                                                   `:9210` |
| `NETATMO_EXPORTER_EXTERNAL_URL` | External URL to use as base for OAuth redirect URL.                        |                                   `http://127.0.0.1:9210` |
|   `NETATMO_EXPORTER_TOKEN_FILE` | Path to token file for loading/persisting authentication token.            | (the Docker image has a default, which can be overridden) |
|                `DEBUG_HANDLERS` | Enables debugging HTTP handlers.                                           |                                                           |
|             `NETATMO_LOG_LEVEL` | Sets the minimum level output through logging.                             |                                                    `info` |
|      `NETATMO_REFRESH_INTERVAL` | Time interval used for internal caching of NetAtmo sensor data.            |                                                      `8m` |
|             `NETATMO_AGE_STALE` | Data age to consider as stale. Stale data does not create metrics anymore. |                                                      `1h` |
|             `NETATMO_CLIENT_ID` | Client ID for NetAtmo app.                                                 |                                                           |
|         `NETATMO_CLIENT_SECRET` | Client secret for NetAtmo app.                                             |                                                           |
|       `NETATMO_ENABLE_HOMECOACH`| Enable Monitoring for AirCare/HomeCoach true or false                      |                                                      true |
|        `NETATMO_ENABLE_WEATHER` | Enable Monitoring for Weather true or false                                |                                                      true |
|     `NETATMO_ENABLE_GO_METRICS` | Enable Monitoring for Go runtime metrics (GC, memory, goroutines) true or false |                                                      false |

### Debugging HTTP handlers

When the `--debug-handlers` flag is set (or the `DEBUG_HANDLERS` environment variable is set to `true`), the exporter will expose additional debugging HTTP handlers on the `/debug/netatmo` endpoint. This can be useful for profiling the application if you experience issues.

The Data will be displayed in the Format  :

```json
{
  "weather": {
    "devices": [
      ...
    ]
  },
  "homecoach": {
    "devices": [
      ...
    ]
  }
}
```

### Cached data

The exporter has an in-memory cache for the data retrieved from the Netatmo API. The purpose of this is to decouple making requests to the Netatmo API from the scraping interval as the data from Netatmo does not update nearly as fast as the default scrape interval of Prometheus. Per the Netatmo documentation the sensor data is updated every ten minutes. The default "refresh interval" of the exporter is set a bit below this (8 minutes), but still much higher than the default Prometheus scrape interval (15 seconds).

You can still set a slower scrape interval for this exporter if you like:

```yml
scrape_configs:
  - job_name: 'netatmo'
    scrape_interval: 90s
    static_configs:
      - targets: ['localhost:9210']
```
