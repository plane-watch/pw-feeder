package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"pw-feeder/lib/atc_status"
	"pw-feeder/lib/connproxy"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

const (
	// ExitcodeConfigError indicates that the feeder configuration is invalid.
	ExitcodeConfigError = 78
)

var (
	// app defines the pw-feeder command-line application.
	app = cli.Command{
		Name:        "pw-feeder",
		Usage:       "feed ADS-B data to plane.watch",
		Description: `Plane Watch Feeder Client`,
		Version:     "0.0.10",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "apikey",
				Usage:    "plane.watch user API Key",
				Required: true,
				Sources:  cli.EnvVars("API_KEY"),
				Action: func(ctx context.Context, command *cli.Command, s string) error {
					// Validate the supplied API key.
					apikey, err := uuid.Parse(command.String("apikey"))
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
				Name:    "beasthost",
				Usage:   "Host to connect to for BEAST data",
				Value:   "127.0.0.1",
				Sources: cli.EnvVars("BEASTHOST"),
			},
			&cli.UintFlag{
				Name:    "beastport",
				Usage:   "TCP port on beasthost to connect to BEAST data",
				Value:   30005,
				Sources: cli.EnvVars("BEASTPORT"),
			},
			&cli.StringFlag{
				Name:    "mlatserverhost",
				Usage:   "Listen host for MLAT server connection",
				Value:   "127.0.0.1",
				Sources: cli.EnvVars("MLATSERVERHOST"),
			},
			&cli.UintFlag{
				Name:    "mlatserverport",
				Usage:   "Listen port for MLAT server connection",
				Value:   12346,
				Sources: cli.EnvVars("MLATSERVERPORT"),
			},
			&cli.StringFlag{
				Name:    "beastout",
				Hidden:  true,
				Usage:   "plane.watch endpoint for BEAST data",
				Value:   "feed.push.plane.watch:12345",
				Sources: cli.EnvVars("PW_BEAST_ENDPOINT"),
			},
			&cli.StringFlag{
				Name:    "mlatout",
				Hidden:  true,
				Usage:   "plane.watch endpoint for MLAT data",
				Value:   "feed.push.plane.watch:12346",
				Sources: cli.EnvVars("PW_MLAT_ENDPOINT"),
			},
			&cli.StringFlag{
				Name:    "atcurl",
				Hidden:  true,
				Usage:   "atc.plane.watch base URL for API calls",
				Value:   "https://atc.plane.watch",
				Sources: cli.EnvVars("PW_ATC_URL"),
			},
			&cli.BoolFlag{
				Name:    "debug",
				Usage:   "Enable debug logging",
				Sources: cli.EnvVars("DEBUG"),
			},
			&cli.BoolFlag{
				Name:    "insecure",
				Usage:   "Skip verifying server TLS certificate",
				Sources: cli.EnvVars("INSECURE"),
			},
			&cli.BoolFlag{
				Name:    "nomlat",
				Usage:   "Disable MLAT",
				Sources: cli.EnvVars("NOMLAT"),
			},
			&cli.BoolFlag{
				Name:    "nocolor",
				Aliases: []string{"nocolour"},
				Usage:   "Disable color output in log",
				Sources: cli.EnvVars("NOCOLOR", "NOCOLOUR"),
			},
		},
		Action: runFeeder,
		Before: func(ctx context.Context, command *cli.Command) (context.Context, error) {

			// Set the log level.
			if !command.Bool("debug") {
				zerolog.SetGlobalLevel(zerolog.InfoLevel)
			}

			// Configure console logging.
			logConfig := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.UnixDate}
			if command.Bool("nocolor") {
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
			redactList := map[string]string{
				command.String("apikey"): "[API_KEY_REDACTED]",
			}
			logConfig.FormatPrepare = redactFromLogs(redactList)

			// Set the global logger.
			log.Logger = log.Output(logConfig)
			return ctx, nil
		},
	}
)

// commithash returns the VCS revision embedded in the binary's build metadata.
func commithash() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return "unknown"
}

// redactFromLogs returns a log formatter that replaces sensitive values.
func redactFromLogs(redactList map[string]string) func(event map[string]interface{}) error {
	return func(event map[string]interface{}) error {
		for k, v := range event {
			vStr, isStr := v.(string)
			if !isStr {
				continue
			}
			for toRedact, redactTo := range redactList {
				vStr = strings.ReplaceAll(vStr, toRedact, redactTo)
			}
			event[k] = vStr
		}
		return nil
	}
}

// main runs the command-line application and reports fatal errors.
func main() {
	err := app.Run(context.Background(), os.Args)
	if err != nil {
		log.Fatal().Err(err).Msg("plane.watch feeder finishing with an error")
	}
}

// runFeeder starts the feeder services and shuts them down on SIGTERM.
func runFeeder(ctx context.Context, command *cli.Command) error {
	var err error

	// Log startup information.
	log.Info().
		Str("commithash", commithash()[:7]).
		Str("version", command.Version).
		Msg("plane.watch feeder started")

	// Set up a cancellable context.
	ctx, cancel := context.WithCancel(ctx)

	// Track the feeder goroutines for a graceful shutdown.
	wg := sync.WaitGroup{}

	// Prepare the MLAT listener.
	var listenMLAT net.Listener
	if !command.Bool("nomlat") {
		listenMLAT, err = net.Listen("tcp", fmt.Sprintf("%s:%d", command.String("mlatserverhost"), command.Uint("mlatserverport")))
		if err != nil {
			cancel()
			return err
		}
		defer func() {
			_ = listenMLAT.Close()
		}()
	}

	// Prepare the signal handler.
	sigTermChan := make(chan os.Signal)
	signal.Notify(sigTermChan, syscall.SIGTERM)

	// Start the BEAST tunnel.
	wg.Go(func() {
		connproxy.ProxyBEASTConnection(
			ctx,
			"BEAST",
			fmt.Sprintf("%s:%d", command.String("beasthost"), command.Uint("beastport")),
			command.String("beastout"),
			command.String("apikey"),
			command.Bool("insecure"),
		)
	})

	// Start the MLAT tunnel.
	if !command.Bool("nomlat") {
		wg.Go(func() {
			connproxy.ProxyMLATConnection(
				ctx,
				"MLAT",
				listenMLAT,
				command.String("mlatout"),
				command.String("apikey"),
				command.Bool("insecure"),
			)
		})
	}

	// Start the status updater.
	wg.Go(func() {
		atc_status.Start(
			ctx,
			command.String("atcurl"),
			command.String("apikey"),
			300,
		)
	})

	// Wait for SIGTERM.
	_ = <-sigTermChan
	log.Info().Msg("received SIGTERM, stopping")

	// Cancel the context to stop the feeder goroutines.
	cancel()
	atc_status.Stop() // The context cancellation should already have stopped it.

	// Wait for the feeder goroutines to finish.
	wg.Wait()

	return nil
}
