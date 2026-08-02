package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

// plane.watch configuration command line flags & env vars
const (
	// flagAPIKey names the CLI flag that supplies the plane.watch API key.
	flagAPIKey = "apikey"
	// envAPIKey names the environment variable that supplies the plane.watch API key.
	envAPIKey = "API_KEY"

	// flagBeastOut names the hidden CLI flag for the plane.watch BEAST endpoint.
	flagBeastOut = "beastout"
	// envBeastOut names the environment variable for the plane.watch BEAST endpoint.
	envBeastOut = "PW_BEAST_ENDPOINT"

	// flagMLATOut names the hidden CLI flag for the plane.watch MLAT endpoint.
	flagMLATOut = "mlatout"
	// envMLATOut names the environment variable for the plane.watch MLAT endpoint.
	envMLATOut = "PW_MLAT_ENDPOINT"

	// flagATCUrl names the hidden CLI flag for the ATC API base URL.
	flagATCUrl = "atcurl"
	// envATCUrl names the environment variable for the ATC API base URL.
	envATCUrl = "PW_ATC_URL"

	// flagInsecure names the CLI flag that disables server TLS verification.
	flagInsecure = "insecure"
	// envInsecure names the environment variable that disables server TLS verification.
	envInsecure = "INSECURE"
)

// Logging configuration command line flags & env vars
const (
	// flagDebug names the CLI flag that enables debug logging and metrics.
	flagDebug = "debug"
	// envDebug names the environment variable that enables debug logging and metrics.
	envDebug = "DEBUG"

	// flagNoColor names the CLI flag that disables colored log output.
	flagNoColor = "nocolor"
	// envNoColor names the environment variable that disables colored log output.
	envNoColor = "NOCOLOR"

	// flagNoColour names the Queen's English alias for flagNoColor.
	flagNoColour = "nocolour"
	// envNoColour names the Queen's English alias for envNoColor.
	envNoColour = "NOCOLOUR"
)

// Metrics configuration command line flags & env vars
const (
	// flagMetricsHost names the CLI flag for the metrics listener host.
	flagMetricsHost = "metricshost"
	// envMetricsHost names the environment variable for the metrics listener host.
	envMetricsHost = "PW_METRICSHOST"

	// flagMetricsPort names the CLI flag for the metrics listener port.
	flagMetricsPort = "metricsport"
	// envMetricsPort names the environment variable for the metrics listener port.
	envMetricsPort = "PW_METRICSPORT"

	// flagNoMetrics names the CLI flag that disables the metrics listener.
	flagNoMetrics = "nometrics"
	// envNoMetrics names the environment variable that disables the metrics listener.
	envNoMetrics = "PW_NOMETRICS"
)

// BEAST data source configuration command line flags & env vars
const (
	// flagBeastHost names the CLI flag for the local BEAST data source host.
	flagBeastHost = "beasthost"
	// envBeastHost names the environment variable for the local BEAST data source host.
	envBeastHost = "BEASTHOST"

	// flagBeastPort names the CLI flag for the local BEAST data source port.
	flagBeastPort = "beastport"
	// envBeastPort names the environment variable for the local BEAST data source port.
	envBeastPort = "BEASTPORT"
)

// Multilateration configuration command line flags & env vars
const (
	// flagMLATServerHost names the CLI flag for the local MLAT listener host.
	flagMLATServerHost = "mlatserverhost"
	// envMLATServerHost names the environment variable for the local MLAT listener host.
	envMLATServerHost = "MLATSERVERHOST"

	// flagMLATServerPort names the CLI flag for the local MLAT listener port.
	flagMLATServerPort = "mlatserverport"
	// envMLATServerPort names the environment variable for the local MLAT listener port.
	envMLATServerPort = "MLATSERVERPORT"

	// flagNoMLAT names the CLI flag that disables multilateration support.
	flagNoMLAT = "nomlat"
	// envNoMLAT names the environment variable that disables multilateration support.
	envNoMLAT = "NOMLAT"
)

var (
	// app defines the pw-feeder command-line application.
	app = cli.Command{
		Name:        "pw-feeder",
		Usage:       "feed ADS-B data to plane.watch",
		Description: `Plane Watch Feeder Client`,
		Version:     feederVersion,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     flagAPIKey,
				Category: "plane.watch:",
				Usage:    "plane.watch user API Key",
				Required: true,
				Sources:  cli.EnvVars(envAPIKey),
				Action: func(ctx context.Context, command *cli.Command, s string) error {
					// Validate the supplied API key.
					apikey, err := uuid.Parse(command.String(flagAPIKey))
					if err != nil {
						return cli.Exit("The API Key provided isn't a valid UUID, please check the arguments or environment file in your docker-compose.yml and try again", ExitcodeConfigError)
					}
					if apikey.String() == "00000000-0000-0000-0000-000000000000" {
						return cli.Exit("The API Key provided is the default API key in the documentation, please update the arguments or environment file in your docker-compose.yml and try again", ExitcodeConfigError)
					}
					return nil
				},
			},
			&cli.StringFlag{
				Name:     flagBeastHost,
				Category: "BEAST Data Source:",
				Usage:    "Host to connect to for BEAST data",
				Value:    "127.0.0.1",
				Sources:  cli.EnvVars(envBeastHost),
			},
			&cli.UintFlag{
				Name:     flagBeastPort,
				Category: "BEAST Data Source:",
				Usage:    "TCP port on beasthost to connect to for BEAST data",
				Value:    30005,
				Sources:  cli.EnvVars(envBeastPort),
			},
			&cli.StringFlag{
				Name:     flagMLATServerHost,
				Category: "Multilateration:",
				Usage:    "Listen host for MLAT server connection",
				Value:    "127.0.0.1",
				Sources:  cli.EnvVars(envMLATServerHost),
			},
			&cli.UintFlag{
				Name:     flagMLATServerPort,
				Category: "Multilateration:",
				Usage:    "Listen port for MLAT server connection",
				Value:    12346,
				Sources:  cli.EnvVars(envMLATServerPort),
			},
			&cli.StringFlag{
				Name:     flagBeastOut,
				Category: "plane.watch:",
				Hidden:   true,
				Usage:    "plane.watch endpoint for BEAST data",
				Value:    "feed.push.plane.watch:12345",
				Sources:  cli.EnvVars(envBeastOut),
			},
			&cli.StringFlag{
				Name:     flagMLATOut,
				Category: "plane.watch:",
				Hidden:   true,
				Usage:    "plane.watch endpoint for MLAT data",
				Value:    "feed.push.plane.watch:12346",
				Sources:  cli.EnvVars(envMLATOut),
			},
			&cli.StringFlag{
				Name:     flagATCUrl,
				Category: "plane.watch:",
				Hidden:   true,
				Usage:    "atc.plane.watch base URL for API calls",
				Value:    "https://atc.plane.watch",
				Sources:  cli.EnvVars(envATCUrl),
			},
			&cli.BoolFlag{
				Name:     flagDebug,
				Category: "Logging:",
				Usage:    "Enable debug logging & metrics",
				Sources:  cli.EnvVars(envDebug),
			},
			&cli.BoolFlag{
				Name:     flagInsecure,
				Category: "plane.watch:",
				Usage:    "Skip verifying server TLS certificate",
				Sources:  cli.EnvVars(envInsecure),
			},
			&cli.BoolFlag{
				Name:     flagNoMLAT,
				Category: "Multilateration:",
				Usage:    "Disable MLAT",
				Sources:  cli.EnvVars(envNoMLAT),
			},
			&cli.BoolFlag{
				Name:     flagNoColor,
				Category: "Logging:",
				Aliases:  []string{flagNoColour},
				Usage:    "Disable color output in log",
				Sources:  cli.EnvVars(envNoColor, envNoColour),
			},
			&cli.StringFlag{
				Name:     flagMetricsHost,
				Category: "Metrics:",
				Usage:    "Listen host for metrics",
				Value:    "127.0.0.1",
				Sources:  cli.EnvVars(envMetricsHost),
			},
			&cli.UintFlag{
				Name:     flagMetricsPort,
				Category: "Metrics:",
				Usage:    "Listen port for metrics",
				Value:    2112,
				Sources:  cli.EnvVars(envMetricsPort),
			},
			&cli.BoolFlag{
				Name:     flagNoMetrics,
				Category: "Metrics:",
				Usage:    "Disable metrics",
				Sources:  cli.EnvVars(envNoMetrics),
			},
		},
		Action: runFeeder,
		Before: func(ctx context.Context, command *cli.Command) (context.Context, error) {

			// Set the log level.
			if !command.Bool(flagDebug) {
				zerolog.SetGlobalLevel(zerolog.InfoLevel)
			}

			// Configure console logging.
			logConfig := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.UnixDate}
			if command.Bool(flagNoColor) {
				logConfig.FormatTimestamp = func(i interface{}) string {
					return fmt.Sprintf("[%s] %v", command.Name, i)
				}
				logConfig.NoColor = true
			} else {
				logConfig.FormatTimestamp = func(i interface{}) string {
					return fmt.Sprintf("[%s] \x1b[%dm%v\x1b[0m", command.Name, 90, i) // 90 = Dark Gray colour
				}
			}

			// Ensure secrets are redacted.
			redactList = make(map[string]string)
			logConfig.FormatPrepare = redactFromLogs(redactList)

			// Set the global logger.
			log.Logger = log.Output(logConfig)
			return ctx, nil
		},
	}
)

// feederConfig contains the runtime configuration used by the feeder services.
type feederConfig struct {
	version string
	apiKey  string

	beastSource   string
	beastEndpoint string

	mlatEnabled  bool
	mlatListen   string
	mlatEndpoint string

	atcURL   string
	insecure bool
	debug    bool

	metricsEnabled bool
	metricsAddress string
}

// configFromCommand snapshots CLI values and returns a feederConfig.
func configFromCommand(command *cli.Command) feederConfig {
	return feederConfig{
		version: command.Version,
		apiKey:  command.String(flagAPIKey),

		beastSource: fmt.Sprintf(
			"%s:%d",
			command.String(flagBeastHost),
			command.Uint(flagBeastPort),
		),
		beastEndpoint: command.String(flagBeastOut),

		mlatEnabled: !command.Bool(flagNoMLAT),
		mlatListen: fmt.Sprintf(
			"%s:%d",
			command.String(flagMLATServerHost),
			command.Uint(flagMLATServerPort),
		),
		mlatEndpoint: command.String(flagMLATOut),

		atcURL:   command.String(flagATCUrl),
		insecure: command.Bool(flagInsecure),
		debug:    command.Bool(flagDebug),

		metricsEnabled: !command.Bool(flagNoMetrics),
		metricsAddress: net.JoinHostPort(
			command.String(flagMetricsHost),
			command.String(flagMetricsPort),
		),
	}
}
