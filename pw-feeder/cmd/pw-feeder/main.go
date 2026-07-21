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
	ExitcodeConfigError = 78
)

var (
	Redactables = make(map[string]string)

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
					// sanity checks on api key entered
					apikey, err := uuid.Parse(command.String("apikey"))
					if err != nil {
						return cli.Exit("The API Key provided isn't a valid UUID, please check the arguments or environment file in your docker-compose.yml and try again", ExitcodeConfigError)
					}
					if apikey.String() == "00000000-0000-0000-0000-000000000000" {
						return cli.Exit("The API Key provided is the default API key in the documentation, please update the arguments or environment file in your docker-compose.yml and try again", ExitcodeConfigError)
					}
					// ensure api key redacted from logs
					Redactables[s] = "[API_KEY_REDACTED]"
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
				Value:   "http://atc.plane.watch",
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

			// set log level
			if !command.Bool("debug") {
				zerolog.SetGlobalLevel(zerolog.InfoLevel)
			}

			// configure logging
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

			// ensure API Key is redacted
			logConfig.FormatPrepare = redactFromLogs

			// set logger
			log.Logger = log.Output(logConfig)
			return ctx, nil
		},
	}
)

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

func redactFromLogs(event map[string]interface{}) error {
	for k, v := range event {
		vStr, isStr := v.(string)
		if !isStr {
			continue
		}
		for toRedact, redactTo := range Redactables {
			event[k] = strings.Replace(vStr, toRedact, redactTo, -1)
		}
	}
	return nil
}

func main() {
	// Run & final exit
	err := app.Run(context.Background(), os.Args)
	if err != nil {
		log.Fatal().Err(err).Msg("plane.watch feeder finishing with an error")
	} else {
		log.Info().Msg("plane.watch feeder finishing without error")
	}
}

func runFeeder(ctx context.Context, command *cli.Command) error {
	var err error

	log.Info().
		Str("commithash", commithash()[:7]).
		Str("version", command.Version).
		Msg("plane.watch feeder started")

	ctx, cancel := context.WithCancel(ctx)
	wg := sync.WaitGroup{}

	// prep mlat listener
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

	// prep signal handler
	sigTermChan := make(chan os.Signal)
	signal.Notify(sigTermChan, syscall.SIGTERM)

	// start beast tunnel
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

	// start MLAT tunnel
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

	// start status updater
	wg.Go(func() {
		atc_status.Start(
			ctx,
			command.String("atcurl"),
			command.String("apikey"),
			300,
		)
	})

	// wait for sigterm
	_ = <-sigTermChan
	log.Info().Msg("received SIGTERM, stopping")
	cancel()
	atc_status.Stop()

	wg.Wait()

	return nil
}
