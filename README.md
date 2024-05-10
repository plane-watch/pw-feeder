# pw-feeder

[![codecov](https://codecov.io/gh/plane-watch/pw-feeder/branch/main/graph/badge.svg?token=8Y55DNDVEE)](https://codecov.io/gh/plane-watch/pw-feeder)

Feeder client for plane.watch.

Tunnels BEAST and MLAT data from your client to plane.watch over a TLS tunnel.

## Runtime Configuration

| Option | Environment Variable | Description | Default |
| ------ | -------------------- | ----------- | ------- |
| `--apikey` | `API_KEY` | plane.watch user API Key | *unset* |
| `--beasthost` | `BEASTHOST` | Host to connect to BEAST data | `127.0.0.1` |
| `--beastport` | `BEASTPORT` | Port to connect to BEAST data | `30005` |
| `--mlatserverhost` | `MLATSERVERHOST` | Listen host for `mlat-client` server connection | `127.0.0.1` |
| `--mlatserverport` | `MLATSERVERPORT` | Listen port for `mlat-client` server connection | `30105` |
| `--debug` | `DEBUG` | Enable debug logging | `false` |

## Installing from Binary


## Building & Installing from Source

* Clone the repo
* Change into the `pw-feeder` directory
* Run `go mod tidy` to download required modules
* Run `go generate ./...` to download required CA certs
* Test: `go test ./...`
* Build & Install: `go build -o /usr/local/bin/pw-feeder ./cmd/pw-feeder`
