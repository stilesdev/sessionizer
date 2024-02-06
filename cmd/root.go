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

            // TODO: figure out how to dynamically change prompt or something to indicate that a generic session will be created with the entered query as the name
            fzf := exec.Command("fzf", "--exact", "--print-query")
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
                // fzf still returns a non-zero exit code when nothing selected, but need to ignore that so this can still read the entered query and create a generic session
                //log.Fatalln(err)
            } 

            selected := strings.TrimSpace(string(fzfResult))

            var sessionName string
            var sessionPath string

            lines := strings.Split(string(fzfResult), "\n")
            fmt.Println("Split result:", lines, len(lines))
            if len(lines) == 2 {
                fmt.Println("Create new generic session with name:", lines[0])
                sessionName = lines[0]
                sessionPath = xdg.Home
            } else if len(lines) == 3 {
                fmt.Println("Selected entry from list:", selected)
                sessionName = filepath.Base(selected)
                sessionPath = lines[1]
            } else {
                log.Fatalln("Invalid result returned from fzf")
            }




            
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
