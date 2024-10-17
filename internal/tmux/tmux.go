package tmux

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"strconv"
	"strings"
)


type TmuxSession struct {
    Name string
    Path string
    Attached bool
    Env map[string]string
    Command string
    Split PaneSplit
	Windows []TmuxWindow
}

type PaneSplit struct {
    Direction string
    Size string
    Command string
    Path string
}

type TmuxWindow struct {
	Path string
	Env map[string]string
	Command string
}

func IsTmuxAvailable() bool {
    if _, err := exec.LookPath("tmux"); err != nil {
        return false
    }

    return true
}

func IsInTmux() bool {
    return os.Getenv("TMUX") != ""
}

func SwitchToSession(session TmuxSession) {
    cmd := exec.Command("tmux", "switch-client", "-t", session.Name)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()
}

func AttachToSession(session TmuxSession) {
    cmd := exec.Command("tmux", "attach", "-t", session.Name)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()
}

func CreateNewSession(session TmuxSession) error {
    args := []string{
        "new-session",
        "-d",
        "-s", session.Name,
        "-c", session.Path,
    }

    for key, val := range session.Env {
        args = append(args, "-e", fmt.Sprintf("%s=%s", key, val))
    }

    cmd := exec.Command("tmux", args...)
    if err := cmd.Run(); err != nil {
        return err
    }

    if session.Command != "" {
        cmd = exec.Command("tmux", "send-keys", "-t", session.Name + ".0", session.Command, "ENTER")
        if err := cmd.Run(); err != nil {
            return err
        }
    }

    if session.Split.Direction != "" && session.Split.Size != "" {
        var direction string
        if session.Split.Direction == "h" || session.Split.Direction == "horizontal" {
            direction = "-h"
        } else if session.Split.Direction == "v" || session.Split.Direction == "vertical" {
            direction = "-v"
        } else {
            return errors.New("Invalid split direction")
        }

        path := session.Path
        if session.Split.Path != "" {
            path = session.Split.Path
        }

        args := []string{
            "split-pane",
            direction,
            "-t", session.Name + ".0",
            "-l", session.Split.Size,
            "-c", path,
        }

        for key, val := range session.Env {
            args = append(args, "-e", fmt.Sprintf("%s=%s", key, val))
        }

        cmd = exec.Command("tmux", args...)
        if err := cmd.Run(); err != nil {
            return err
        }

        if session.Split.Command != "" {
            cmd = exec.Command("tmux", "send-keys", "-t", session.Name + ".1", session.Split.Command, "ENTER")
            if err := cmd.Run(); err != nil {
                return err
            }
        }

        cmd = exec.Command("tmux", "select-pane", "-t", session.Name + ".0")
        if err := cmd.Run(); err != nil {
            return err
        }
    }

	for i, window := range session.Windows {
		path := session.Path
		if window.Path != "" {
			path = window.Path
		}

		args := []string{
			"new-window",
			"-a", "-t", session.Name + ":" + strconv.Itoa(i),
			"-d", // don't make the new window the active window
			"-c", path,
		}

		// merge base session env with window env, window env takes priority on collisions
		env := maps.Clone(session.Env)
		maps.Copy(env, window.Env)

		for key, val := range env {
			args = append(args, "-e", fmt.Sprintf("%s=%s", key, val))
		}

		cmd = exec.Command("tmux", args...)
        if err := cmd.Run(); err != nil {
            return err
        }

		if window.Command != "" {
            cmd = exec.Command("tmux", "send-keys", "-t", session.Name + ":" + strconv.Itoa(i+1), window.Command, "ENTER")
            if err := cmd.Run(); err != nil {
                return err
            }
		}
	}

    return nil
}

func ListExistingSessions() ([]TmuxSession, error) {
    cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name} #{session_path} #{session_attached}")
    sessionOut, err := cmd.StdoutPipe()
    if err != nil {
        return nil, err
    }
    cmd.Stderr = os.Stderr

    if err = cmd.Start(); err != nil {
        return nil, err
    }

    sessionOutContents, err := io.ReadAll(sessionOut)
    if err != nil {
        return nil, err
    }

    var existingSessions []TmuxSession

    if err = cmd.Wait(); err != nil {
        // return empty slice with nil error here as tmux is probably just not running, which shouldn't be considered an error
        return existingSessions, nil
    }

    sessions := strings.TrimSpace(string(sessionOutContents))

    for _, sessionString := range strings.Split(string(sessions), "\n") {
        split := strings.Split(sessionString, " ")

        if len(split) == 3 {
            existingSessions = append(existingSessions, TmuxSession{
                Name: split[0],
                Path: split[1],
                Attached: split[2] == "1",
            })
        }
    }

    return existingSessions, nil
}
