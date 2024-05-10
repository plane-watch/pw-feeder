package stunnel

import (
	"embed"
)

// Let's Encrypt Root CAs

// ISRG Root X1, Validity: until 2030-06-04 (generated 2015-06-04)
//go:generate curl --no-progress-meter -o isrg-root-x1.pem https://letsencrypt.org/certs/isrgrootx1.pem

// ISRG Root X2, Validity: until 2035-09-04 (generated 2020-09-04)
//go:generate curl --no-progress-meter -o isrg-root-x2.pem https://letsencrypt.org/certs/isrg-root-x2.pem

// Let's Encrypt Subordinate (Intermediate) CAs

// Let’s Encrypt E5, Validity: until 2027-03-12
//go:generate curl --no-progress-meter -o lets-encrypt-e5.pem https://letsencrypt.org/certs/2024/e5.pem

// Let’s Encrypt E6, Validity: until 2027-03-12
//go:generate curl --no-progress-meter -o lets-encrypt-e6.pem https://letsencrypt.org/certs/2024/e6.pem

// Let’s Encrypt R10, Validity: until 2027-03-12
//go:generate curl --no-progress-meter -o lets-encrypt-r10.pem https://letsencrypt.org/certs/2024/r10.pem

// Let’s Encrypt R11, Validity: until 2027-03-12
//go:generate curl --no-progress-meter -o lets-encrypt-r11.pem https://letsencrypt.org/certs/2024/r11.pem

//go:embed *.pem
var caCertPEMs embed.FS
