package cmd

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"github.com/stilesdev/sessionizer/internal/fzf"
	"github.com/stilesdev/sessionizer/internal/tmux"
)

type SessionsConfig struct {
    Path string
    Paths []string
}

type TmuxConfig struct {
    HideAttachedSessions bool
}

type Config struct {
    Sessions []SessionsConfig
    Tmux TmuxConfig
}

func parseGlobToPaths(glob string) ([]string) {
    var paths []string

    if strings.HasPrefix(glob, "~/") {
        glob = filepath.Join(xdg.Home, glob[2:])
    }

    matches, err := filepath.Glob(glob)
    if err != nil {
        fmt.Println("Unable to parse glob:", glob)
        return paths
    }

    for _, path := range matches {
        paths = append(paths, path)
    }

    return paths
}

func tmuxSessionExists(path string, existingSessions []tmux.TmuxSession) bool {
    for _, existingSession := range existingSessions {
        if existingSession.Path == path {
            return true
        }
    }
    
    return false
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

            for _, sessionConfig := range config.Sessions {
                if sessionConfig.Path != "" {
                    for _, path := range parseGlobToPaths(sessionConfig.Path) {
                        if !tmuxSessionExists(path, existingSessions) {
                            fuzzyFindEntries = append(fuzzyFindEntries, path)
                        }
                    }
                }

                if len(sessionConfig.Paths) > 0 {
                    for _, glob := range sessionConfig.Paths {
                        for _, path := range parseGlobToPaths(glob) {
                            if !tmuxSessionExists(path, existingSessions) {
                                fuzzyFindEntries = append(fuzzyFindEntries, path)
                            }
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
        defaultSessionConfig := SessionsConfig{
            Path: filepath.Join(xdg.Home, "*"),
        }
        config.Sessions = append(config.Sessions, defaultSessionConfig)
    }
}
