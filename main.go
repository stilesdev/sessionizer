package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/stilesdev/sessionizer/internal/config"
	"github.com/stilesdev/sessionizer/internal/fzf"
	"github.com/stilesdev/sessionizer/internal/log"
	"github.com/stilesdev/sessionizer/internal/session"
	"github.com/stilesdev/sessionizer/internal/tmux"
	"github.com/urfave/cli/v2"
)

func main() {
	var userConfigFile string
	var sessionOverride string

	defaultConfigFile, err := config.GetDefaultConfigFile()
	log.FatalIfError(err)

	cli := cli.App{
		Name: "sessionizer",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config",
				Aliases:     []string{"c"},
				Usage:       "Load configuration from `FILE`",
				Value:       defaultConfigFile,
				Destination: &userConfigFile,
			},
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Print debug messages to stdout",
				Action: func(ctx *cli.Context, value bool) error {
					log.ShowDebugMessages(value)
					return nil
				},
			},
			&cli.StringFlag{
				Name:        "open",
				Usage:       "Open the provided session immediately instead of prompting to select a session with fzf",
				Destination: &sessionOverride,
			},
		},
		Action: func(ctx *cli.Context) error {
			err := checkPathDependencies()
			if err != nil {
				return err
			}

			conf, err := config.Parse(userConfigFile)
			if err != nil {
				return err
			}

			log.Debug(fmt.Sprintf("Loaded config: %+v", conf))

			existingSessions, err := tmux.ListExistingSessions()
			if err != nil {
				return err
			}

			sessions := session.BuildSessionList(conf, existingSessions, sessionOverride)

			var selected session.Session

			if sessionOverride != "" {
				selected, err = session.SelectFromOverride(sessions, sessionOverride)
			} else {
				selected, err = session.SelectFromFzfPrompt(sessions)
			}

			if err != nil {
				if errors.Is(err, session.AbortedError) {
					// not an error, but no valid selection made
					log.Debug(err)
					return nil
				}

				return err
			}

			log.Debug(fmt.Sprintf("Selected: %#v", selected))

			return tmux.ActivateSession(selected)
		},
	}

	if err := cli.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func checkPathDependencies() error {
	if !tmux.IsAvailable() {
		return errors.New("tmux is not installed or could not be found in $PATH")
	}

	if !fzf.IsAvailable() {
		return errors.New("fzf is not installed or could not be found in $PATH")
	}

	return nil
}
