# pw-feeder

Feeder client for plane.watch.

Tunnels BEAST and MLAT data from your client to plane.watch over a TLS tunnel.

## Runtime Configuration

| Option | Environment Variable | Description | Default |
| ------ | -------------------- | ----------- | ------- |
| `--apikey` | `API_KEY` | plane.watch user API Key | *unset* |
| `--beasthost` | `BEASTHOST` | Host to connect to BEAST data | `127.0.0.1` |
| `--beastport` | `BEASTPORT` | Port to connect to BEAST data | `30005` |
| `--mlatresultshost` | `MLATRESULTSHOST` | Host to connect to for `mlat-client` results in BEAST format | `127.0.0.1` |
| `--mlatresultsport` | `MLATRESULTSPORT` | Port to connect to for `mlat-client` results in BEAST format | `30105` |
| `--debug` | `DEBUG` | Enable debug logging | `false` |

## Building & Installing

* Clone the repo
* Change into the `pw-feeder` directory
* Run `go mod tidy` to download required modules
* Build: `go build ./...`
* Install: `go install ./...`
