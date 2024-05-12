# pw-feeder

[![codecov](https://codecov.io/gh/plane-watch/pw-feeder/branch/main/graph/badge.svg?token=8Y55DNDVEE)](https://codecov.io/gh/plane-watch/pw-feeder)

Feeder client for plane.watch.

Tunnels BEAST and MLAT data from your client to plane.watch over a TLS tunnel.

## Runtime Configuration

| Option             | Environment Variable | Description                                     | Default     |
|--------------------|----------------------|-------------------------------------------------|-------------|
| `--apikey`         | `API_KEY`            | plane.watch user API Key                        | *unset*     |
| `--beasthost`      | `BEASTHOST`          | Host to connect to BEAST data                   | `127.0.0.1` |
| `--beastport`      | `BEASTPORT`          | Port to connect to BEAST data                   | `30005`     |
| `--mlatserverhost` | `MLATSERVERHOST`     | Listen host for `mlat-client` server connection | `127.0.0.1` |
| `--mlatserverport` | `MLATSERVERPORT`     | Listen port for `mlat-client` server connection | `30105`     |
| `--debug`          | `DEBUG`              | Enable debug logging                            | `false`     |

## Installing from Binary


## Building & Installing from Source

* Clone the repo
* Change into the `pw-feeder` directory
* Run `go mod tidy` to download required modules
* Test: `go test ./...`
* Build & Install: `go build -o /usr/local/bin/pw-feeder ./cmd/pw-feeder`

## Installing CA Certificates

If you receive an error `x509: certificate signed by unknown authority` when `pw-feeder` attempts to connect, you will need to install the [Let's Encrypt CA certificates](https://letsencrypt.org/certificates/).

We provide a helper script to do this on Debian and Ubuntu flavours of Linux. This script can be execute with the following command:

```bash
curl https://raw.githubusercontent.com/plane-watch/pw-feeder/remove_embedded_ca/install_ca_certs.sh | bash
```

The script uses `sudo`, so you will be prompted to enter your password. If you'd prefer to do this manually, the commands are shown and explained below.

| Command                                                                                         | Explanation                                                                               |
|-------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------|
| `sudo mkdir -p /usr/share/ca-certificates/letsencrypt`                                          | Create the directory `/usr/share/ca-certificates/letsencrypt` to hold the CA certificates |
| `cd /usr/share/ca-certificates/letsencrypt`                                                     | Change into the directory `/usr/share/ca-certificates/letsencrypt`                        |
| `sudo curl -o isrg-root-x1.crt https://letsencrypt.org/certs/isrgrootx1.pem`                    | Download the **ISRG Root X1** certificate                                                 |
| `sudo curl -o isrg-root-x2.crt https://letsencrypt.org/certs/isrg-root-x2.pem`                  | Download the **ISRG Root X2** certificate                                                 |
| `sudo curl -o lets-encrypt-e5.crt https://letsencrypt.org/certs/2024/e5.pem`                    | Download the **Let’s Encrypt E5** certificate                                             |
| `sudo curl -o lets-encrypt-e6.crt https://letsencrypt.org/certs/2024/e6.pem`                    | Download the **Let’s Encrypt E6** certificate                                             |
| `sudo curl -o lets-encrypt-e7.crt https://letsencrypt.org/certs/2024/e7.pem`                    | Download the **Let’s Encrypt E7** certificate                                             |
| `sudo curl -o lets-encrypt-e8.crt https://letsencrypt.org/certs/2024/e8.pem`                    | Download the **Let’s Encrypt E8** certificate                                             |
| `sudo curl -o lets-encrypt-e9.crt https://letsencrypt.org/certs/2024/e9.pem`                    | Download the **Let’s Encrypt E9** certificate                                             |
| `sudo curl -o lets-encrypt-r10.crt https://letsencrypt.org/certs/2024/r10.pem`                  | Download the **Let’s Encrypt R10** certificate                                            |
| `sudo curl -o lets-encrypt-r11.crt https://letsencrypt.org/certs/2024/r11.pem`                  | Download the **Let’s Encrypt R11** certificate                                            |
| `sudo curl -o lets-encrypt-r12.crt https://letsencrypt.org/certs/2024/r12.pem`                  | Download the **Let’s Encrypt R12** certificate                                            |
| `sudo curl -o lets-encrypt-r13.crt https://letsencrypt.org/certs/2024/r13.pem`                  | Download the **Let’s Encrypt R13** certificate                                            |
| `sudo curl -o lets-encrypt-r14.crt https://letsencrypt.org/certs/2024/r14.pem`                  | Download the **Let’s Encrypt R14** certificate                                            |
| `sudo curl -o lets-encrypt-e1.crt https://letsencrypt.org/certs/lets-encrypt-e1.pem`            | Download the **Let’s Encrypt E1** certificate                                             |
| `sudo curl -o lets-encrypt-e2.crt https://letsencrypt.org/certs/lets-encrypt-e2.pem`            | Download the **Let’s Encrypt E2** certificate                                             |
| `sudo curl -o lets-encrypt-r3.crt https://letsencrypt.org/certs/lets-encrypt-r3.pem`            | Download the **Let’s Encrypt R3** certificate                                             |
| `sudo curl -o lets-encrypt-r4.crt https://letsencrypt.org/certs/lets-encrypt-r4.pem`            | Download the **Let’s Encrypt R4** certificate                                             |
| `cd /usr/share/ca-certificates`                                                                 | Change into the directory `/usr/share/ca-certificates`                                    |
| `find letsencrypt/ -maxdepth 1 -type f -iname '*.crt' \| sudo tee -a /etc/ca-certificates.conf` | Append the newly downloaded certificates to `/etc/ca-certificates.conf`                   |
| `sudo update-ca-certificates`                                                                   | Regenerates ca-certificates.crt, a concatenated single-file list of CA certificates.      |
