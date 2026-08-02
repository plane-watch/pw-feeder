package main

import (
	"context"
	"os"
	"runtime/debug"
	"strings"

	"github.com/rs/zerolog/log"
)

const (

	// feederVersion is the version of the feeder (duh).
	feederVersion = "0.0.11"

	// ExitcodeConfigError indicates that the feeder configuration is invalid.
	ExitcodeConfigError = 78
)

var (
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
