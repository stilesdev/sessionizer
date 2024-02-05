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
                files, err := os.ReadDir(localDir)
                if err != nil {
                    log.Fatalln(err)
                }

                for _, file := range files {
                    if file.IsDir() {
                        fuzzyFindEntries = append(fuzzyFindEntries, localDir + "/" + file.Name())
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




            
            if _, err := exec.LookPath("tmux"); err != nil {
                log.Fatalln("tmux is not installed or could not be found in $PATH")
            }

            inTmux := os.Getenv("TMUX") != ""

            tmux := exec.Command("tmux", "list-sessions", "-F", "#{session_name} #{session_path} #{session_attached}")
            fmt.Println(tmux.String())
            sessionOut, err := tmux.StdoutPipe()
            if err != nil {
                 log.Fatalln(err)
            }

            sessionErr, err := tmux.StderrPipe()
            if err != nil {
                 log.Fatalln(err)
            }

            if err = tmux.Start(); err != nil {
                log.Fatalln(err)
            }

            sessionOutContents, err := io.ReadAll(sessionOut)
            if err != nil {
                log.Fatalln(err)
            }

            sessionErrContents, err := io.ReadAll(sessionErr)
            if err != nil {
                log.Fatalln(err)
            }

            tmuxRunning := true
            err = tmux.Wait()
            if err != nil {
                // error could be caused by tmux server not running, which is okay as this will create a new session next. Check stderr for "no server running" message
                sessionErrOutputString := strings.TrimSpace(string(sessionErrContents))
                if strings.Contains(sessionErrOutputString, "no server running") {
                    // tmux server not running, remember this so it can skip parsing this command's output later
                    tmuxRunning = false
                } else {
                    log.Fatalln(err)
                }
            }

            sessions := strings.TrimSpace(string(sessionOutContents))

            var existingSession string

            if tmuxRunning {
                for _, session := range strings.Split(string(sessions), "\n") {
                    split := strings.Split(session, " ")
                    name := split[0]
                    path := split[1]
                    attached := split[2] == "1"

                    if sessionName == name {
                        fmt.Printf("Found existing session with name: %v, path: %v, isAttached: %v\n", name, path, attached)
                        existingSession = name
                        break
                    }
                }
            }

            if existingSession == "" {
                fmt.Println("Creating new session")
                // selected session does not exist, create it now
                createSession := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-c", selected)
                fmt.Println(createSession.String())
                if err = createSession.Run(); err != nil {
                    log.Fatalln(err)
                }
                existingSession = sessionName
            }
            
            if inTmux {
                // switch to session
                cmd := exec.Command("tmux", "switch-client", "-t", existingSession)
                fmt.Println(cmd.String())
                cmd.Stdin = os.Stdin
                cmd.Stdout = os.Stdout
                cmd.Stderr = os.Stderr
                cmd.Run()
            } else {
                // attach to session
                cmd := exec.Command("tmux", "attach", "-t", existingSession)
                fmt.Println(cmd.String())
                cmd.Stdin = os.Stdin
                cmd.Stdout = os.Stdout
                cmd.Stderr = os.Stderr
                cmd.Run()
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
