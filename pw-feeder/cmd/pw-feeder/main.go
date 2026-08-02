package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
				Category: "plane.watch:",
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
				Name:     "beasthost",
				Category: "BEAST Data Source:",
				Usage:    "Host to connect to for BEAST data",
				Value:    "127.0.0.1",
				Sources:  cli.EnvVars("BEASTHOST"),
			},
			&cli.UintFlag{
				Name:     "beastport",
				Category: "BEAST Data Source:",
				Usage:    "TCP port on beasthost to connect to BEAST data",
				Value:    30005,
				Sources:  cli.EnvVars("BEASTPORT"),
			},
			&cli.StringFlag{
				Name:     "mlatserverhost",
				Category: "Multilateration:",
				Usage:    "Listen host for MLAT server connection",
				Value:    "127.0.0.1",
				Sources:  cli.EnvVars("MLATSERVERHOST"),
			},
			&cli.UintFlag{
				Name:     "mlatserverport",
				Category: "Multilateration:",
				Usage:    "Listen port for MLAT server connection",
				Value:    12346,
				Sources:  cli.EnvVars("MLATSERVERPORT"),
			},
			&cli.StringFlag{
				Name:     "beastout",
				Category: "plane.watch:",
				Hidden:   true,
				Usage:    "plane.watch endpoint for BEAST data",
				Value:    "feed.push.plane.watch:12345",
				Sources:  cli.EnvVars("PW_BEAST_ENDPOINT"),
			},
			&cli.StringFlag{
				Name:     "mlatout",
				Category: "plane.watch:",
				Hidden:   true,
				Usage:    "plane.watch endpoint for MLAT data",
				Value:    "feed.push.plane.watch:12346",
				Sources:  cli.EnvVars("PW_MLAT_ENDPOINT"),
			},
			&cli.StringFlag{
				Name:     "atcurl",
				Category: "plane.watch:",
				Hidden:   true,
				Usage:    "atc.plane.watch base URL for API calls",
				Value:    "https://atc.plane.watch",
				Sources:  cli.EnvVars("PW_ATC_URL"),
			},
			&cli.BoolFlag{
				Name:     "debug",
				Category: "Logging:",
				Usage:    "Enable debug logging & metrics",
				Sources:  cli.EnvVars("DEBUG"),
			},
			&cli.BoolFlag{
				Name:     "insecure",
				Category: "plane.watch:",
				Usage:    "Skip verifying server TLS certificate",
				Sources:  cli.EnvVars("INSECURE"),
			},
			&cli.BoolFlag{
				Name:     "nomlat",
				Category: "Multilateration:",
				Usage:    "Disable MLAT",
				Sources:  cli.EnvVars("NOMLAT"),
			},
			&cli.BoolFlag{
				Name:     "nocolor",
				Category: "Logging:",
				Aliases:  []string{"nocolour"},
				Usage:    "Disable color output in log",
				Sources:  cli.EnvVars("NOCOLOR", "NOCOLOUR"),
			},
			&cli.StringFlag{
				Name:     "metricshost",
				Category: "Metrics:",
				Usage:    "Listen host for metrics",
				Value:    "127.0.0.1",
				Sources:  cli.EnvVars("METRICSHOST"),
			},
			&cli.UintFlag{
				Name:     "metricsport",
				Category: "Metrics:",
				Usage:    "Listen port for metrics",
				Value:    2112,
				Sources:  cli.EnvVars("METRICSPORT"),
			},
			&cli.BoolFlag{
				Name:     "nometrics",
				Category: "Metrics:",
				Usage:    "Disable metrics",
				Sources:  cli.EnvVars("NOMETRICS"),
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
			redactList = make(map[string]string)
			logConfig.FormatPrepare = redactFromLogs(redactList)

			// Set the global logger.
			log.Logger = log.Output(logConfig)
			return ctx, nil
		},
	}

	// redactList contains a list of strings to redact from logs
	redactList map[string]string
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
	var (
		err           error
		reg           *prometheus.Registry
		metricsServer *http.Server
	)
	reg = nil
	metricsErr := make(chan error, 1)

	redactList[command.String("apikey")] = "[API_KEY_REDACTED]"

	// Log startup information.
	log.Info().
		Str("commithash", commithash()[:7]).
		Str("version", command.Version).
		Msg("plane.watch feeder started")

	// Set up a cancellable context.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Track the feeder goroutines for a graceful shutdown.
	wg := sync.WaitGroup{}

	// Create prometheus registry & listener
	if !command.Bool("nometrics") {

		reg = prometheus.NewRegistry()
		if command.Bool("debug") {
			err = reg.Register(collectors.NewGoCollector())
			if err != nil {
				return fmt.Errorf("could not register metrics collector: %w", err)
			}
			err = reg.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
			if err != nil {
				return fmt.Errorf("could not register process collector: %w", err)
			}
		}

		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

		metricsAddr := net.JoinHostPort(command.String("metricshost"), command.String("metricsport"))

		metricsServer = &http.Server{
			Addr:              metricsAddr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			BaseContext: func(net.Listener) context.Context {
				return ctx
			},
		}

	}

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
	sigTermChan := make(chan os.Signal, 1)
	signal.Notify(sigTermChan, syscall.SIGTERM)
	defer signal.Stop(sigTermChan)

	// Start the metrics listener after all fallible listener setup succeeds.
	if metricsServer != nil {
		go func() {
			err := metricsServer.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				metricsErr <- err
			}
		}()

		log.Info().Msgf("Metrics available at http://%s/metrics", metricsServer.Addr)
	}

	// Start the BEAST tunnel.
	wg.Go(func() {
		connproxy.ProxyBEASTConnection(
			ctx,
			"BEAST",
			fmt.Sprintf("%s:%d", command.String("beasthost"), command.Uint("beastport")),
			command.String("beastout"),
			command.String("apikey"),
			command.Bool("insecure"),
			reg,
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
				reg,
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
			reg,
		)
	})

	var runErr error
	select {
	case <-sigTermChan:
		log.Info().Msg("received SIGTERM, stopping")
	case err := <-metricsErr:
		runErr = fmt.Errorf("metrics listener failed: %w", err)
	}
	cancel()

	// stop metrics server
	if metricsServer != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		err := metricsServer.Shutdown(shutdownCtx)
		shutdownCancel()
		if err != nil && runErr == nil {
			runErr = fmt.Errorf("could not stop metrics listener: %w", err)
		}
	}

	// Wait for the feeder goroutines to finish.
	wg.Wait()

	return runErr
}
