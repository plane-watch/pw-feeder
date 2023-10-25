package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:        "pw-feeder",
		Description: `Plane Watch Feeder Client`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "apikey",
				Usage:    "plane.watch user API Key",
				Required: true,
				EnvVars:  []string{"API_KEY"},
			},
			&cli.StringFlag{
				Name:    "beasthost",
				Usage:   "Host to connect to BEAST data",
				Value:   "127.0.0.1",
				EnvVars: []string{"BEASTHOST"},
			},
			&cli.UintFlag{
				Name:    "beastport",
				Usage:   "Port to connect to BEAST data",
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
		},
		Action: runFeeder,
	}

	// get version from git info
	var commithash = func() string {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					return setting.Value
				}
			}
		}
		return ""
	}()
	app.Version = commithash[:7]

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

	// Final exit
	if err := app.Run(os.Args); nil != err {
		log.Err(err).Msg("plane.watch feeder finishing with an error")
		os.Exit(1)
	}

}

func runFeeder(ctx *cli.Context) error {
	log.Info().Str("version", ctx.App.Version).Msg("plane.watch feeder started")

	// sanity checks on api key entered
	apikey, err := uuid.Parse(ctx.String("apikey"))
	if err != nil {
		return errors.New("The API Key provided isn't a valid UUID, please check the arguments or environment file in your docker-compose.yml and try again")
	}

	if apikey.String() == "00000000-0000-0000-0000-000000000000" {
		return errors.New("The API Key provided is the default API key in the documentation, please update the arguments or environment file in your docker-compose.yml and try again")
	}

	wg := sync.WaitGroup{}

	// start beast tunnel
	wg.Add(1)
	go tunnelOutboundConnection(
		"BEAST",
		fmt.Sprintf("%s:%s", ctx.String("beasthost"), ctx.String("beastport")),
		ctx.String("beastout"),
		ctx.String("apikey"),
		wg.Done,
	)

	// start MLAT tunnel
	wg.Add(1)
	go tunnelInboundConnection(
		"MLAT",
		fmt.Sprintf("%s:%s", ctx.String("mlatserverhost"), ctx.String("mlatserverport")),
		ctx.String("mlatout"),
		ctx.String("apikey"),
		wg.Done,
	)

	// start status updater
	wg.Add(1)
	go initStatusUpdater(
		ctx.String("atcurl"),
		ctx.String("apikey"),
		wg.Done,
	)

	wg.Wait()

	return nil
}
