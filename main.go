package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
	"github.com/stilesdev/sessionizer/internal/fzf"
	"github.com/stilesdev/sessionizer/internal/tmux"
	"github.com/urfave/cli/v2"
)

func main() {
    var configFile string
    var config Config

    defaultConfigFile, err := xdg.ConfigFile("sessionizer/config.toml")
    if err != nil {
        log.Fatalln(err)
    }

    cli := cli.App{
        Name: "sessionizer",
        Flags: []cli.Flag{
            &cli.StringFlag{
                Name: "config",
                Aliases: []string{"c"},
                Usage: "Load configuration from `FILE`",
                Value: defaultConfigFile,
                Destination: &configFile,
            },
        },
        Action: func(ctx *cli.Context) error {
            if _, err := toml.DecodeFile(configFile, &config); err == nil {
                fmt.Println("Using config file:", configFile)
                fmt.Printf("%+v\n", config)
            } else {
                if configFile != defaultConfigFile {
                    // user specified config file but we couldn't decode it
                    return err
                }
                fmt.Println("Error:", err)

                // not able to load default config, set defaults for anything required here:
                defaultSessionConfig := SessionsConfig{
                    Path: filepath.Join(xdg.Home, "*"),
                }
                config.Sessions = append(config.Sessions, defaultSessionConfig)
            }


            var sessions []Session

            if !tmux.IsTmuxAvailable() {
                return errors.New("tmux is not installed or could not be found in $PATH")
            }

            if !fzf.IsAvailable() {
                return errors.New("fzf is not installed or could not be found in $PATH")
            }

            fmt.Printf("%#v\n", config)

            existingTmuxSessions, err := tmux.ListExistingSessions()
            if err != nil {
                return err
            }

            for _, sessionConfig := range config.Sessions {
                if sessionConfig.Path != "" {
                    for _, path := range parseGlobToPaths(sessionConfig.Path) {
                        session := parseSession(path, existingTmuxSessions)
                        if !session.IsAttached || !config.Tmux.HideAttachedSessions {
                            sessions = append(sessions, session)
                        }
                    }
                }

                if len(sessionConfig.Paths) > 0 {
                    for _, glob := range sessionConfig.Paths {
                        for _, path := range parseGlobToPaths(glob) {
                            session := parseSession(path, existingTmuxSessions)
                            if !session.IsAttached || !config.Tmux.HideAttachedSessions {
                                sessions = append(sessions, session)
                            }
                        }
                    }
                }
            }

            // look for any scratch sessions - existing sessions in tmux but not associated with any paths from config file
            // TODO: add scratch path as config opt, and explicitly match scratch sessions to that path
            for _, existingSession := range existingTmuxSessions {
                excludeSession := false

                // exclude if attached and configured to hide attached sessions
                if config.Tmux.HideAttachedSessions && existingSession.Attached {
                    excludeSession = true
                }
                
                // exclude if already included via paths - only looking for scratch sessions here
                for _, session := range sessions {
                    if existingSession.Name == session.Name && existingSession.Path == session.Path {
                        excludeSession = true
                        break
                    }
                }

                if !excludeSession {
                    sessions = append(sessions, Session{
                        Path: existingSession.Path,
                        Name: existingSession.Name,
                        FzfEntry: fmt.Sprintf("scratch: %s", existingSession.Name),
                        Exists: true,
                        IsAttached: existingSession.Attached,
                        IsScratch: true,
                    })
                }
            }

            sortSessions(&sessions)

            fzfEntries := make([]string, len(sessions))
            for idx, session := range sessions {
                fzfEntries[idx] = session.FzfEntry
            }

            selectedIndex, _, enteredQuery, err := fzf.Prompt(fzfEntries)
            if err != nil {
                return err
            } else if selectedIndex < 0 && enteredQuery == "" {
                // not an error, but no valid selection made
                fmt.Println("user exited")
                return nil
            }

            var tmuxSession tmux.TmuxSession
            if selectedIndex >= 0 {
                fmt.Printf("%#v\n", sessions[selectedIndex])
                
                if sessions[selectedIndex].Exists {
                    for _, existingTmuxSession := range existingTmuxSessions {
                        if existingTmuxSession.Name == sessions[selectedIndex].Name {
                            tmuxSession = existingTmuxSession
                        }
                    }
                } else {
                    // session does not exist, create it now
                    tmuxSession = tmux.TmuxSession{
                        Name: sessions[selectedIndex].Name,
                        Path: sessions[selectedIndex].Path,
                    }

                    tmux.CreateNewSession(tmuxSession)
                }
            } else {
                // no selection, create scratch
                tmuxSession = tmux.TmuxSession{
                    Name: enteredQuery,
                    Path: xdg.Home,
                }

                tmux.CreateNewSession(tmuxSession)
            }

            if tmux.IsInTmux() {
                tmux.SwitchToSession(tmuxSession)
            } else {
                tmux.AttachToSession(tmuxSession)
            }

            return nil
        },
    }

    if err := cli.Run(os.Args); err != nil {
        log.Fatalln(err)
    }
}

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

type Session struct {
    Path string
    Name string
    FzfEntry string
    Exists bool
    IsScratch bool
    IsAttached bool
}

func parseGlobToPaths(glob string) ([]string) {
    var paths []string

    matches, err := filepath.Glob(expandHome(glob))
    if err != nil {
        fmt.Println("Unable to parse glob:", glob)
        return paths
    }

    for _, path := range matches {
        if fileInfo, err := os.Stat(path); err == nil && fileInfo.IsDir() {
            paths = append(paths, path)
        }
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

func expandHome(path string) string {
    if strings.HasPrefix(path, "~" + string(os.PathSeparator)) {
        return filepath.Join(xdg.Home, path[2:])
    }

    return path
}

func unexpandHome(path string) string {
    if strings.HasPrefix(path, xdg.Home) {
        return filepath.Join("~", path[len(xdg.Home):])
    }

    return path
}

func parseSession(path string, existingTmuxSessions []tmux.TmuxSession) Session {
    session := Session{
        Path: path,
        Name: filepath.Base(path),
        FzfEntry: unexpandHome(path),
    }

    for _, tmuxSession := range existingTmuxSessions {
        if tmuxSession.Path == path && tmuxSession.Name == session.Name {
            session.FzfEntry = fmt.Sprintf("tmux: %s [%s]", session.Name, unexpandHome(path))
            session.Exists = true
            session.IsAttached = tmuxSession.Attached
            break
        }
    }

    return session
}

func sortSessions(sessions *[]Session) {
    slices.SortStableFunc(*sessions, func(a Session, b Session) int {
        if a.IsScratch != b.IsScratch {
            if a.IsScratch {
                return 1
            } else {
                return -1
            }
        }

        if a.Exists != b.Exists {
            if a.Exists {
                return 1
            } else {
                return -1
            }
        }

        return 0
    })
}
