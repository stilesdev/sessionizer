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

	"github.com/stilesdev/sessionizer/internal/config"
	"github.com/stilesdev/sessionizer/internal/session"
)

func IsAvailable() bool {
	if _, err := exec.LookPath("tmux"); err != nil {
		return false
	}

	return true
}

func isInTmux() bool {
	return os.Getenv("TMUX") != ""
}

func switchToSession(session session.Session) {
	cmd := exec.Command("tmux", "switch-client", "-t", session.Name)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func attachToSession(session session.Session) {
	cmd := exec.Command("tmux", "attach", "-t", session.Name)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func createNewSession(session session.Session) error {
	args := []string{
		"new-session",
		"-d",
		"-s", session.Name,
		"-c", session.Config.Path,
	}

	for key, val := range session.Config.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, val))
	}

	cmd := exec.Command("tmux", args...)
	if err := cmd.Run(); err != nil {
		return err
	}

	if session.Config.Command != "" {
		cmd = exec.Command("tmux", "send-keys", "-t", session.Name+".0", session.Config.Command, "ENTER")
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	if session.Config.Split.Direction != "" && session.Config.Split.Size != "" {
		var direction string
		if session.Config.Split.Direction == "h" || session.Config.Split.Direction == "horizontal" {
			direction = "-h"
		} else if session.Config.Split.Direction == "v" || session.Config.Split.Direction == "vertical" {
			direction = "-v"
		} else {
			return errors.New("Invalid split direction")
		}

		path := session.Config.Path
		if session.Config.Split.Path != "" {
			path = session.Config.Split.Path
		}

		args := []string{
			"split-pane",
			direction,
			"-t", session.Name + ".0",
			"-l", session.Config.Split.Size,
			"-c", path,
		}

		// merge base session env with split env, split env takes priority on collisions
		env := maps.Clone(session.Config.Env)
		maps.Copy(env, session.Config.Split.Env)

		for key, val := range env {
			args = append(args, "-e", fmt.Sprintf("%s=%s", key, val))
		}

		cmd = exec.Command("tmux", args...)
		if err := cmd.Run(); err != nil {
			return err
		}

		if session.Config.Split.Command != "" {
			cmd = exec.Command("tmux", "send-keys", "-t", session.Name+".1", session.Config.Split.Command, "ENTER")
			if err := cmd.Run(); err != nil {
				return err
			}
		}

		cmd = exec.Command("tmux", "select-pane", "-t", session.Name+".0")
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	for i, window := range session.Config.Windows {
		path := session.Config.Path
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
		env := maps.Clone(session.Config.Env)
		maps.Copy(env, window.Env)

		for key, val := range env {
			args = append(args, "-e", fmt.Sprintf("%s=%s", key, val))
		}

		cmd = exec.Command("tmux", args...)
		if err := cmd.Run(); err != nil {
			return err
		}

		if window.Command != "" {
			cmd = exec.Command("tmux", "send-keys", "-t", session.Name+":"+strconv.Itoa(i+1), window.Command, "ENTER")
			if err := cmd.Run(); err != nil {
				return err
			}
		}
	}

	return nil
}

func ActivateSession(s session.Session) error {
	if !s.Exists {
		err := createNewSession(s)
		if err != nil {
			return err
		}
	}

	if isInTmux() {
		switchToSession(s)
	} else {
		attachToSession(s)
	}

	return nil
}

func ListExistingSessions() ([]session.Session, error) {
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

	var existingSessions []session.Session

	if err = cmd.Wait(); err != nil {
		// return empty slice with nil error here as tmux is probably just not running, which shouldn't be considered an error
		return existingSessions, nil
	}

	sessions := strings.TrimSpace(string(sessionOutContents))

	for _, sessionString := range strings.Split(string(sessions), "\n") {
		split := strings.Split(sessionString, " ")

		if len(split) == 3 {
			existingSessions = append(existingSessions, session.Session{
				Name:       split[0],
				IsAttached: split[2] == "1",
				Config: config.Session{
					Path: split[1],
				},
			})
		}
	}

	return existingSessions, nil
}
