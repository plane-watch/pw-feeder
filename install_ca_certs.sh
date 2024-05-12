#!/usr/bin/env bash
set -xe
sudo mkdir -p /usr/share/ca-certificates/letsencrypt
pushd /usr/share/ca-certificates/letsencrypt
sudo curl -o isrg-root-x1.crt https://letsencrypt.org/certs/isrgrootx1.pem
sudo curl -o isrg-root-x2.crt https://letsencrypt.org/certs/isrg-root-x2.pem
sudo curl -o lets-encrypt-e5.crt https://letsencrypt.org/certs/2024/e5.pem
sudo curl -o lets-encrypt-e6.crt https://letsencrypt.org/certs/2024/e6.pem
sudo curl -o lets-encrypt-e7.crt https://letsencrypt.org/certs/2024/e7.pem
sudo curl -o lets-encrypt-e8.crt https://letsencrypt.org/certs/2024/e8.pem
sudo curl -o lets-encrypt-e9.crt https://letsencrypt.org/certs/2024/e9.pem
sudo curl -o lets-encrypt-r10.crt https://letsencrypt.org/certs/2024/r10.pem
sudo curl -o lets-encrypt-r11.crt https://letsencrypt.org/certs/2024/r11.pem
sudo curl -o lets-encrypt-r12.crt https://letsencrypt.org/certs/2024/r12.pem
sudo curl -o lets-encrypt-r13.crt https://letsencrypt.org/certs/2024/r13.pem
sudo curl -o lets-encrypt-r14.crt https://letsencrypt.org/certs/2024/r14.pem
sudo curl -o lets-encrypt-e1.crt https://letsencrypt.org/certs/lets-encrypt-e1.pem
sudo curl -o lets-encrypt-e2.crt https://letsencrypt.org/certs/lets-encrypt-e2.pem
sudo curl -o lets-encrypt-r3.crt https://letsencrypt.org/certs/lets-encrypt-r3.pem
sudo curl -o lets-encrypt-r4.crt https://letsencrypt.org/certs/lets-encrypt-r4.pem
pushd /usr/share/ca-certificates
find letsencrypt/ -type f -iname '*.crt' | sudo tee -a /etc/ca-certificates.conf
popd
popd
sudo update-ca-certificates
