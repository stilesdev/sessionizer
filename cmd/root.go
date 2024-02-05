package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
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

            selected, err := io.ReadAll(stdout)
            if err != nil {
                 log.Fatalln(err)
            }

            if err = fzf.Wait(); err != nil {
                log.Fatalln(err)
            } 

            fmt.Println("Selected option:", strings.TrimSpace(string(selected)))
        },
    }
)

func Execute() error {
    return rootCmd.Execute()
}

func init() {
    cobra.OnInitialize(initConfig)

    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf("config file (default: %s/sessionizer/config.toml)", xdg.ConfigHome))
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
