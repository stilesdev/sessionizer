package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/adrg/xdg"
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

			sessions, err := session.MergeSessionLists(existingSessions, conf, sessionOverride)

			var selectedIndex int
			var enteredQuery string

			if sessionOverride == "" {
				fzfEntries := make([]string, len(sessions))
				for idx, session := range sessions {
					fzfEntries[idx] = session.FzfEntry()
				}

				selectedIndex, _, enteredQuery, err = fzf.Prompt(fzfEntries)
				if err != nil {
					return err
				} else if selectedIndex < 0 && enteredQuery == "" {
					// not an error, but no valid selection made
					log.Debug("user exited")
					return nil
				}
			} else {
				for index, session := range sessions {
					if session.Name == sessionOverride || session.Path == sessionOverride {
						selectedIndex = index
						break
					}
				}
			}

			var targetSession session.Session
			if selectedIndex >= 0 {
				log.Debug(fmt.Sprintf("Selected: %#v", sessions[selectedIndex]))
				// use selected session
				targetSession = sessions[selectedIndex]
			} else if enteredQuery != "" {
				// no selection, create scratch
				targetSession = session.Session{
					Name:       enteredQuery,
					Path:       xdg.Home,
					Exists:     false,
					IsAttached: false,
					IsScratch:  true,
				}
			} else {
				log.Debug("no session selected and no query returned from fzf")
				return nil
			}

			return tmux.ActivateSession(targetSession)
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
