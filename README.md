# pw-feeder

Feeder client for plane.watch.

Tunnels BEAST and MLAT data from your client to plane.watch over a TLS tunnel.

## Quick Start

Before running `pw-feeder`, you need:

* A feeder API key. Create a feeder at [atc.plane.watch](https://atc.plane.watch); the [Plane Watch getting-started guide](https://www.plane.watch/getting_started/) explains the registration and API-key creation process.
* A running BEAST data source, such as [readsb](https://github.com/wiedehopf/readsb) or dump1090. By default, `pw-feeder` connects to `127.0.0.1:30005`.
* `mlat-client` configured to connect to `pw-feeder` on `127.0.0.1:12346`, unless MLAT is disabled with `--nomlat` or `NOMLAT=true`.

After installing the binary, start the feeder with the API key supplied as an option or environment variable:

```bash
API_KEY='<your-feeder-api-key>' pw-feeder
```

## Runtime Configuration

| Option                        | Environment Variable      | Description                                                               | Default     |
|-------------------------------|---------------------------|---------------------------------------------------------------------------|-------------|
| `--apikey`                    | `API_KEY`                 | plane.watch feeder API key                                                | *unset*     |
| `--beasthost`                 | `BEASTHOST`               | Host to connect to for BEAST data                                         | `127.0.0.1` |
| `--beastport`                 | `BEASTPORT`               | TCP port to connect to for BEAST data                                     | `30005`     |
| `--mlatserverhost`            | `MLATSERVERHOST`          | Listen host for the `mlat-client` connection                              | `127.0.0.1` |
| `--mlatserverport`            | `MLATSERVERPORT`          | Listen port for the `mlat-client` connection                              | `12346`     |
| `--nomlat`                    | `NOMLAT`                  | Disable MLAT functionality                                                | `false`     |
| `--metricshost`               | `PW_METRICSHOST`          | Listen host for the Prometheus metrics endpoint                           | `127.0.0.1` |
| `--metricsport`               | `PW_METRICSPORT`          | Listen port for the Prometheus metrics endpoint                           | `2112`      |
| `--nometrics`                 | `PW_NOMETRICS`            | Disable the Prometheus metrics endpoint                                   | `false`     |
| `--debug`                     | `DEBUG`                   | Enable debug logging and Go/process metrics                               | `false`     |
| `--nocolor`<br>`--nocolour`   | `NOCOLOR`<br>`NOCOLOUR`   | Disable colour in logs                                                    | `false`     |
| `--insecure`                  | `INSECURE`                | **Testing only:** disable TLS certificate and server identity verification | `false`     |

Prometheus metrics are enabled by default at `http://127.0.0.1:2112/metrics`. Use `--metricshost` and `--metricsport` to change the listener, or `--nometrics` to disable it. The endpoint does not require authentication, so bind it only to a trusted interface or network.

> **WARNING**
> `--insecure` disables verification of the remote server's certificate and identity. Use it only for controlled testing; it makes the TLS connection vulnerable to impersonation and man-in-the-middle attacks.

## Installing from Binary

Pre-built binaries are available from [GitHub Releases](https://github.com/plane-watch/pw-feeder/releases) for these Linux architectures:

| Architecture | Typical systems                                      |
|--------------|------------------------------------------------------|
| `386`        | 32-bit Intel/AMD                                    |
| `amd64`      | 64-bit Intel/AMD (`x86_64`)                         |
| `arm`        | 32-bit ARM, including older Raspberry Pi systems   |
| `arm64`      | 64-bit ARM (`aarch64`), including newer Raspberry Pi systems |

The current release is `v0.0.11`.

For example, to install `v0.0.11` on a 64-bit Intel or AMD Linux system:

```bash
VERSION=v0.0.11
ARCH=amd64
curl -fLO "https://github.com/plane-watch/pw-feeder/releases/download/${VERSION}/pw-feeder.${VERSION}.linux.${ARCH}.tar.xz"
tar -xJf "pw-feeder.${VERSION}.linux.${ARCH}.tar.xz"
sudo install -m 0755 pw-feeder /usr/local/bin/pw-feeder
pw-feeder --version
```

Set `ARCH` to the appropriate value from the table for other supported Linux systems.

## Building & Installing from Source

```bash
git clone https://github.com/plane-watch/pw-feeder.git
cd pw-feeder/pw-feeder
go mod download
go test ./...
go build -o pw-feeder ./cmd/pw-feeder
sudo install -m 0755 pw-feeder /usr/local/bin/pw-feeder
```

## Running as a Docker Container

The [`plane-watch/docker-plane-watch`](https://github.com/plane-watch/docker-plane-watch) project packages `pw-feeder`, `mlat-client`, and their supporting services as a multi-architecture container. The `latest` image supports 32-bit and 64-bit Intel/AMD and ARM systems.

Set `BEASTHOST` to an IP address, hostname, or container name that is reachable from inside the container; `127.0.0.1` refers to the container itself. `LAT`, `LONG`, and `ALT` describe the antenna location and are required for MLAT.

```bash
docker run \
  --detach \
  --name planewatch \
  --restart unless-stopped \
  --env TZ='Australia/Perth' \
  --env BEASTHOST='<beast-source-host>' \
  --env API_KEY='<your-feeder-api-key>' \
  --env LAT='<antenna-latitude>' \
  --env LONG='<antenna-longitude>' \
  --env ALT='<antenna-altitude-with-m-or-ft-suffix>' \
  --tmpfs=/run:exec,size=64M \
  --tmpfs=/var/log \
  ghcr.io/plane-watch/docker-plane-watch:latest
```

No host ports need to be published. To disable MLAT in the container, add `--env ENABLE_MLAT=false` to the command.

Follow the container logs with:

```bash
docker logs --follow planewatch
```

Confirm that the container is receiving BEAST data with:

```bash
docker exec --interactive --tty planewatch viewadsb
```

See the [container repository](https://github.com/plane-watch/docker-plane-watch) for Docker Compose instructions, image tags, and the complete list of environment variables.

## Running with systemd

The included [`pw-feeder.service`](pw-feeder.service) runs the feeder as an unprivileged `pw-feeder` system user and reads its configuration from `/etc/pw-feeder/pw-feeder.env`.

Create the service account and configuration file:

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin pw-feeder
sudo install -d -m 0755 /etc/pw-feeder
sudo touch /etc/pw-feeder/pw-feeder.env
sudo chmod 0600 /etc/pw-feeder/pw-feeder.env
sudoedit /etc/pw-feeder/pw-feeder.env
```

At minimum, add your API key to the environment file. Configure the BEAST source if it is not listening on the default address:

```dotenv
API_KEY=<your-feeder-api-key>
BEASTHOST=127.0.0.1
BEASTPORT=30005
```

Install and start the service:

```bash
sudo install -m 0644 pw-feeder.service /etc/systemd/system/pw-feeder.service
sudo systemctl daemon-reload
sudo systemctl enable --now pw-feeder
sudo systemctl status pw-feeder
```

View its logs with:

```bash
sudo journalctl -u pw-feeder -f
```

## Installing CA Certificates

If you receive an error `x509: certificate signed by unknown authority` when `pw-feeder` attempts to connect, you will need to install the current [Let's Encrypt CA certificates](https://letsencrypt.org/certificates/).

For example:
```bash
sudo apt update
sudo apt install --reinstall ca-certificates
sudo update-ca-certificates
```
