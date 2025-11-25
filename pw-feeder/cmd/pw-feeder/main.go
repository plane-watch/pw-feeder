package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"pw-feeder/lib/atc_status"
	"pw-feeder/lib/connproxy"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

var (
	app = &cli.App{
		Name:        "pw-feeder",
		Usage:       "feed ADS-B data to plane.watch",
		Description: `Plane Watch Feeder Client`,
		Version:     "0.0.6",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "apikey",
				Usage:    "plane.watch user API Key",
				Required: true,
				EnvVars:  []string{"API_KEY"},
			},
			&cli.StringFlag{
				Name:    "beasthost",
				Usage:   "Host to connect to for BEAST data",
				Value:   "127.0.0.1",
				EnvVars: []string{"BEASTHOST"},
			},
			&cli.UintFlag{
				Name:    "beastport",
				Usage:   "TCP port on beasthost to connect to BEAST data",
				Value:   30005,
				EnvVars: []string{"BEASTPORT"},
			},
			&cli.StringFlag{
				Name:    "mlatserverhost",
				Usage:   "Listen host for MLAT server connection",
				Value:   "127.0.0.1",
				EnvVars: []string{"MLATSERVERHOST"},
			},
			&cli.UintFlag{
				Name:    "mlatserverport",
				Usage:   "Listen port for MLAT server connection",
				Value:   12346,
				EnvVars: []string{"MLATSERVERPORT"},
			},
			&cli.StringFlag{
				Name:    "beastout",
				Hidden:  true,
				Usage:   "plane.watch endpoint for BEAST data",
				Value:   "feed.push.plane.watch:12345",
				EnvVars: []string{"PW_BEAST_ENDPOINT"},
			},
			&cli.StringFlag{
				Name:    "mlatout",
				Hidden:  true,
				Usage:   "plane.watch endpoint for MLAT data",
				Value:   "feed.push.plane.watch:12346",
				EnvVars: []string{"PW_MLAT_ENDPOINT"},
			},
			&cli.StringFlag{
				Name:    "atcurl",
				Hidden:  true,
				Usage:   "atc.plane.watch base URL for API calls",
				Value:   "http://atc.plane.watch",
				EnvVars: []string{"PW_ATC_URL"},
			},
			&cli.BoolFlag{
				Name:    "debug",
				Usage:   "Enable debug logging",
				EnvVars: []string{"DEBUG"},
			},
			&cli.BoolFlag{
				Name:    "insecure",
				Hidden:  true,
				Usage:   "Skip verify of server certificate",
				EnvVars: []string{"INSECURE"},
			},
			&cli.BoolFlag{
				Name:    "nomlat",
				Usage:   "Disable MLAT",
				EnvVars: []string{"NOMLAT"},
			},
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

func main() {
	app.Action = runFeeder

	// configure logging
	logConfig := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.UnixDate}
	logConfig.FormatTimestamp = func(i interface{}) string {
		return fmt.Sprintf("[%s] \x1b[%dm%v\x1b[0m", app.Name, 90, i) // 90 = Dark Gray colour
	}
	log.Logger = log.Output(logConfig)

	// Set logging level
	app.Before = func(c *cli.Context) error {
		if !c.Bool("debug") {
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
		}
		return nil
	}

	// Run & final exit
	err := app.Run(os.Args)
	if err != nil {
		log.Err(err).Msg("plane.watch feeder finishing with an error")
		os.Exit(1)
	} else {
		log.Info().Msg("plane.watch feeder finishing without error")
	}

}

func runFeeder(cliContext *cli.Context) error {
	log.Info().
		Str("commithash", commithash()[:7]).
		Str("version", app.Version).
		Msg("plane.watch feeder started")

	// sanity checks on api key entered
	apikey, err := uuid.Parse(cliContext.String("apikey"))
	if err != nil {
		return errors.New("The API Key provided isn't a valid UUID, please check the arguments or environment file in your docker-compose.yml and try again")
	}

	if apikey.String() == "00000000-0000-0000-0000-000000000000" {
		return errors.New("The API Key provided is the default API key in the documentation, please update the arguments or environment file in your docker-compose.yml and try again")
	}

	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}

	// prep mlat listener
	var listenMLAT net.Listener
	if !cliContext.Bool("nomlat") {
		listenMLAT, err = net.Listen("tcp", fmt.Sprintf("%s:%s", cliContext.String("mlatserverhost"), cliContext.String("mlatserverport")))
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
			fmt.Sprintf("%s:%s", cliContext.String("beasthost"), cliContext.String("beastport")),
			cliContext.String("beastout"),
			cliContext.String("apikey"),
			cliContext.Bool("insecure"),
		)
	})

	// start MLAT tunnel
	if !cliContext.Bool("nomlat") {
		wg.Go(func() {
			connproxy.ProxyMLATConnection(
				ctx,
				"MLAT",
				listenMLAT,
				cliContext.String("mlatout"),
				cliContext.String("apikey"),
				cliContext.Bool("insecure"),
			)
		})
	}

	// start status updater
	wg.Go(func() {
		atc_status.Start(
			ctx,
			cliContext.String("atcurl"),
			cliContext.String("apikey"),
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
