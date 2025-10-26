#!/usr/bin/env bash
set -xe
sudo mkdir -p /usr/share/ca-certificates/letsencrypt
cd /usr/share/ca-certificates/letsencrypt
sudo curl -o isrg-root-x1.crt https://letsencrypt.org/certs/isrgrootx1.pem
sudo curl -o isrg-root-x2.crt https://letsencrypt.org/certs/isrg-root-x2.pem
sudo curl -o isrg-root-ye.crt https://letsencrypt.org/certs/gen-y/root-ye.pem
sudo curl -o isrg-root-yr.crt https://letsencrypt.org/certs/gen-y/root-yr.pem
sudo curl -o lets-encrypt-e7.crt https://letsencrypt.org/certs/2024/e7.pem
sudo curl -o lets-encrypt-e8.crt https://letsencrypt.org/certs/2024/e8.pem
sudo curl -o lets-encrypt-r12.crt https://letsencrypt.org/certs/2024/r12.pem
sudo curl -o lets-encrypt-r13.crt https://letsencrypt.org/certs/2024/r13.pem
cd /usr/share/ca-certificates
find letsencrypt/ -maxdepth 1 -type f -iname '*.crt' | sudo tee -a /etc/ca-certificates.conf
sudo update-ca-certificates
