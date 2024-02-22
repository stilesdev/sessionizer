package tmux

import (
	"io"
	"os"
	"os/exec"
	"strings"
)


type TmuxSession struct {
    Name string
    Path string
    Attached bool
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
    //fmt.Println(cmd.String())
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()
}

func AttachToSession(session TmuxSession) {
    cmd := exec.Command("tmux", "attach", "-t", session.Name)
    //fmt.Println(cmd.String())
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()
}

func CreateNewSession(session TmuxSession) error {
    cmd := exec.Command("tmux", "new-session", "-d", "-s", session.Name, "-c", session.Path)
    //fmt.Println(cmd String())
    if err := cmd.Run(); err != nil {
        return err
    }

    return nil
}

func ListExistingSessions() ([]TmuxSession, error) {
    cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name} #{session_path} #{session_attached}")
    //fmt.Println(cmd.String())
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
