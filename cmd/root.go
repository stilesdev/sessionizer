package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stilesdev/sessionizer/multiplexers/tmux"
	"github.com/stilesdev/sessionizer/prompts/fzf"
)

var (
    cfgFile string

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

            hideAttachedSessions := viper.GetBool("tmux.hideAttachedSessions")
            hideExistingSessionsMatchingLocalPath := viper.GetBool("tmux.hideExistingSessionsMatchingLocalPath")

            localDirs := viper.GetStringSlice("localDirs")

            for _, localDir := range localDirs {
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
                        if !hideExistingSessionsMatchingLocalPath {
                            for _, existingSession := range existingSessions {
                                if existingSession.Path == fullPath {
                                    excludeDir = true
                                }
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

                if hideAttachedSessions && existingSession.Attached {
                    excludeSession = true
                }

                if !excludeSession && hideExistingSessionsMatchingLocalPath {
                    for _, entry := range fuzzyFindEntries {
                        if existingSession.Path == entry {
                            excludeSession = true
                        }
                    }
                }

                if !excludeSession {
                    existingSessionEntries = append(existingSessionEntries, tmuxEntryPrefix + existingSession.Name)
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
                if !isTmuxSession {
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

    viper.SetConfigFile(cfgFile)
    viper.SetConfigType("toml")

    if err := viper.ReadInConfig(); err == nil {
        fmt.Println("Using config file:", viper.ConfigFileUsed())
    } else {
        fmt.Println("Error:", err)

        // not able to load config, set defaults for anything required here:
        viper.Set("localDirs", []string{xdg.Home})
    }
}
