package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stilesdev/sessionizer/multiplexers/tmux"
)

var (
    cfgFile string

    rootCmd = &cobra.Command{
        Use: "sessionizer",
        Short: "A smart terminal session manager",
        Run: func(cmd *cobra.Command, args []string) {
            var fuzzyFindEntries []string

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
                        fuzzyFindEntries = append(fuzzyFindEntries, filepath.Join(localDir, file.Name()))
                    }
                }
            }

            if _, err := exec.LookPath("fzf"); err != nil {
                log.Fatalln("fzf is not installed or could not be found in $PATH")
            }

            fzf := exec.Command("fzf")
            stdin, err := fzf.StdinPipe()
            if err != nil {
                 log.Fatalln(err)
            }
            defer stdin.Close()

            fzf.Stderr = os.Stderr

            stdout, err := fzf.StdoutPipe()
            if err != nil {
                 log.Fatalln(err)
            }
            defer stdout.Close()

            if err = fzf.Start(); err != nil {
                log.Fatalln(err)
            }

            _, err = io.WriteString(stdin, strings.Join(fuzzyFindEntries, "\n"))
            if err != nil {
                 log.Fatalln(err)
            }

            fzfResult, err := io.ReadAll(stdout)
            if err != nil {
                 log.Fatalln(err)
            }

            if err = fzf.Wait(); err != nil {
                log.Fatalln(err)
            } 

            selected := strings.TrimSpace(string(fzfResult))
            fmt.Println("Selected option:", selected)

            // TODO: detect which type of option was selected before running this - file paths only are supported for now
            sessionName := filepath.Base(selected)




            
            if !tmux.IsTmuxAvailable() {
                log.Fatalln("tmux is not installed or could not be found in $PATH")
            }

            existingSessions, err := tmux.ListExistingSessions()
            if err != nil {
                log.Fatalln(err)
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
                    Path: selected,
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
