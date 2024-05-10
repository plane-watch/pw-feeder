package stunnel

import (
	"embed"
)

// Let's Encrypt CAs

//go:generate curl --progress-bar -w %{filename_effective}\n -O https://letsencrypt.org/certs/isrgrootx1.pem
//go:generate curl --progress-bar -w %{filename_effective}\n -O https://letsencrypt.org/certs/isrg-root-x2.pem
//go:generate curl --progress-bar -w %{filename_effective}\n -O https://letsencrypt.org/certs/2024/e5.pem
//go:generate curl --progress-bar -w %{filename_effective}\n -O https://letsencrypt.org/certs/2024/e6.pem
//go:generate curl --progress-bar -w %{filename_effective}\n -O https://letsencrypt.org/certs/2024/e7.pem
//go:generate curl --progress-bar -w %{filename_effective}\n -O https://letsencrypt.org/certs/2024/e8.pem
//go:generate curl --progress-bar -w %{filename_effective}\n -O https://letsencrypt.org/certs/2024/e9.pem
//go:generate curl --progress-bar -w %{filename_effective}\n -O https://letsencrypt.org/certs/2024/r10.pem
//go:generate curl --progress-bar -w %{filename_effective}\n -O https://letsencrypt.org/certs/2024/r11.pem
//go:generate curl --progress-bar -w %{filename_effective}\n -O https://letsencrypt.org/certs/2024/r12.pem
//go:generate curl --progress-bar -w %{filename_effective}\n -O https://letsencrypt.org/certs/2024/r13.pem
//go:generate curl --progress-bar -w %{filename_effective}\n -O https://letsencrypt.org/certs/2024/r14.pem
//go:generate curl --progress-bar -w %{filename_effective}\n -O https://letsencrypt.org/certs/lets-encrypt-e1.pem
//go:generate curl --progress-bar -w %{filename_effective}\n -O https://letsencrypt.org/certs/lets-encrypt-e2.pem
//go:generate curl --progress-bar -w %{filename_effective}\n -O https://letsencrypt.org/certs/lets-encrypt-r3.pem
//go:generate curl --progress-bar -w %{filename_effective}\n -O https://letsencrypt.org/certs/lets-encrypt-r4.pem

//go:embed *.pem
var caCertPEMs embed.FS
