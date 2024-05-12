#!/usr/bin/env bash
set -xe
sudo mkdir -p /usr/share/ca-certificates/letsencrypt
cd /usr/share/ca-certificates/letsencrypt
sudo curl --no-clobber -o isrg-root-x1.crt https://letsencrypt.org/certs/isrgrootx1.pem
sudo curl --no-clobber -o isrg-root-x2.crt https://letsencrypt.org/certs/isrg-root-x2.pem
sudo curl --no-clobber -o e5.crt https://letsencrypt.org/certs/2024/e5.pem
sudo curl --no-clobber -o e6.crt https://letsencrypt.org/certs/2024/e6.pem
sudo curl --no-clobber -o e7.crt https://letsencrypt.org/certs/2024/e7.pem
sudo curl --no-clobber -o e8.crt https://letsencrypt.org/certs/2024/e8.pem
sudo curl --no-clobber -o e9.crt https://letsencrypt.org/certs/2024/e9.pem
sudo curl --no-clobber -o r10.crt https://letsencrypt.org/certs/2024/r10.pem
sudo curl --no-clobber -o r11.crt https://letsencrypt.org/certs/2024/r11.pem
sudo curl --no-clobber -o r12.crt https://letsencrypt.org/certs/2024/r12.pem
sudo curl --no-clobber -o r13.crt https://letsencrypt.org/certs/2024/r13.pem
sudo curl --no-clobber -o r14.crt https://letsencrypt.org/certs/2024/r14.pem
sudo curl --no-clobber -o lets-encrypt-e1.crt https://letsencrypt.org/certs/lets-encrypt-e1.pem
sudo curl --no-clobber -o lets-encrypt-e2.crt https://letsencrypt.org/certs/lets-encrypt-e2.pem
sudo curl --no-clobber -o lets-encrypt-r3.crt https://letsencrypt.org/certs/lets-encrypt-r3.pem
sudo curl --no-clobber -o lets-encrypt-r4.crt https://letsencrypt.org/certs/lets-encrypt-r4.pem
sudo update-ca-certificates
