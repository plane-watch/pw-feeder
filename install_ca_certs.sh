#!/usr/bin/env bash
# Copyright (C) 2024 Plane Watch
# SPDX-License-Identifier: GPL-3.0-or-later
#
# This file is part of pw-feeder.
#
# pw-feeder is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# pw-feeder is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with pw-feeder. If not, see <https://www.gnu.org/licenses/>.

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
sudo curl -o lets-encrypt-ye1.crt https://letsencrypt.org/certs/gen-y/int-ye1.pem
sudo curl -o lets-encrypt-ye2.crt https://letsencrypt.org/certs/gen-y/int-ye2.pem
sudo curl -o lets-encrypt-yr1.crt https://letsencrypt.org/certs/gen-y/int-yr1.pem
sudo curl -o lets-encrypt-yr2.crt https://letsencrypt.org/certs/gen-y/int-yr2.pem
cd /usr/share/ca-certificates
find letsencrypt/ -maxdepth 1 -type f -iname '*.crt' | sudo tee -a /etc/ca-certificates.conf
sudo update-ca-certificates
