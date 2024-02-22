package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"github.com/stilesdev/sessionizer/internal/fzf"
	"github.com/stilesdev/sessionizer/internal/tmux"
)

type TmuxConfig struct {
    HideAttachedSessions bool
}

type Config struct {
    Tmux TmuxConfig
    LocalDirs []string
}

var (
    cfgFile string
    config Config

    rootCmd = &cobra.Command{
        Use: "sessionizer",
        Short: "A smart terminal session manager",
        Run: func(cmd *cobra.Command, args []string) {
            var fuzzyFindEntries []string

            if !tmux.IsTmuxAvailable() {
                log.Fatalln("tmux is not installed or could not be found in $PATH")
            }

            existingSessions, err := tmux.ListExistingSessions()
            if err != nil {
                log.Fatalln(err)
            }

            for _, localDir := range config.LocalDirs {
                if strings.HasPrefix(localDir, "~/") {
                    localDir = filepath.Join(xdg.Home, localDir[2:])
                }

                files, err := os.ReadDir(localDir)
                if err != nil {
                    log.Fatalln(err)
                }

                for _, file := range files {
                    if file.IsDir() {
                        fullPath := filepath.Join(localDir, file.Name())
                        excludeDir := false

                        for _, existingSession := range existingSessions {
                            if existingSession.Path == fullPath {
                                excludeDir = true
                            }
                        }

                        if !excludeDir {
                            fuzzyFindEntries = append(fuzzyFindEntries, fullPath)
                        }
                    }
                }
            }

            tmuxEntryPrefix := "tmux: "
            var existingSessionEntries []string
            for _, existingSession := range existingSessions {
                excludeSession := false

                if config.Tmux.HideAttachedSessions && existingSession.Attached {
                    excludeSession = true
                }

                if !excludeSession {
                    existingSessionEntries = append(existingSessionEntries, tmuxEntryPrefix + existingSession.Name + " [" + existingSession.Path + "]")
                }
            }

            fuzzyFindEntries = append(fuzzyFindEntries, existingSessionEntries...)



            if !fzf.IsAvailable() {
                log.Fatalln("fzf is not installed or could not be found in $PATH")
            }

            selectedOption, enteredQuery, err := fzf.Prompt(fuzzyFindEntries)
            if err != nil {
                log.Fatalln(err)
            }

            var sessionName string
            var sessionPath string

            if selectedOption != "" {
                fmt.Println("Selected entry from list:", selectedOption)

                var isTmuxSession bool
                sessionName, isTmuxSession = strings.CutPrefix(selectedOption, tmuxEntryPrefix)
                if isTmuxSession {
                    pathDelimIndex := strings.Index(sessionName, " [")
                    if pathDelimIndex > 0 {
                        sessionName = sessionName[:pathDelimIndex]
                    }
                } else {
                    sessionName = filepath.Base(selectedOption)
                    sessionPath = selectedOption
                }
            } else {
                fmt.Println("Create new generic session with name:", enteredQuery)
                sessionName = enteredQuery
                sessionPath = xdg.Home
            }




            
            var session tmux.TmuxSession

            for _, existingSession := range existingSessions {
                if existingSession.Name == sessionName {
                    session = existingSession
                }
            }

            if session.Name == "" {
                // selected session does not exist, create it now
                session = tmux.TmuxSession{
                    Name: sessionName,
                    Path: sessionPath,
                    Attached: false,
                }

                tmux.CreateNewSession(session)
            }
            
            if tmux.IsInTmux() {
                tmux.SwitchToSession(session)
            } else {
                tmux.AttachToSession(session)
            }
        },
    }
)

func Execute() error {
    return rootCmd.Execute()
}

func init() {
    cobra.OnInitialize(initConfig)

    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf("config file (default: %v/sessionizer/config.toml)", xdg.ConfigHome))
}

func initConfig() {
    if cfgFile == "" {
        var err error
        cfgFile, err = xdg.ConfigFile("sessionizer/config.toml")
        cobra.CheckErr(err)
    }

    if _, err := toml.DecodeFile(cfgFile, &config); err == nil {
        fmt.Println("Using config file:", cfgFile)
        fmt.Printf("%+v\n", config)
    } else {
        fmt.Println("Error:", err)

        // not able to load config, set defaults for anything required here:
        config.LocalDirs = []string{xdg.Home}
    }
}
