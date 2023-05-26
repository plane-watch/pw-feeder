package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	app := &cli.App{
		Version:     "20230526",
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
				Usage:   "Host to connect to for MLAT server connection",
				Value:   "127.0.0.1",
				EnvVars: []string{"MLATSERVERHOST"},
			},
			&cli.UintFlag{
				Name:    "mlatserverport",
				Usage:   "Port to connect to for MLAT server connection",
				Value:   30105,
				EnvVars: []string{"MLATSERVERPORT"},
			},
			&cli.StringFlag{
				Name:    "beastout",
				Hidden:  true,
				Usage:   "plane.watch endpoint for BEAST data",
				Value:   "feed.push.plane.watch:22345",
				EnvVars: []string{"PW_BEAST_ENDPOINT"},
			},
			&cli.StringFlag{
				Name:    "mlatout",
				Hidden:  true,
				Usage:   "plane.watch endpoint for MLAT data",
				Value:   "feed.push.plane.watch:22346",
				EnvVars: []string{"PW_MLAT_ENDPOINT"},
			},
			&cli.BoolFlag{
				Name:    "debug",
				Usage:   "Enable debug logging",
				EnvVars: []string{"DEBUG"},
			},
		},
		Action: runFeeder,
	}

	// configure logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

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
	go tunnelOutboundConnection(
		"MLAT",
		fmt.Sprintf("%s:%s", ctx.String("mlatserverhost"), ctx.String("mlatserverport")),
		ctx.String("mlatout"),
		ctx.String("apikey"),
		wg.Done,
	)

	wg.Wait()

	return nil
}
