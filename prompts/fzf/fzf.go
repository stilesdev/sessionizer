package fzf

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
)

func IsAvailable() bool {
    if _, err := exec.LookPath("fzf"); err != nil {
        return false
    }

    return true
}

func Prompt(options []string) (string, string, error) {
    // TODO: figure out how to dynamically change prompt or something to indicate that a generic session will be created with the entered query as the name
    fzf := exec.Command("fzf", "--exact", "--print-query")
    stdin, err := fzf.StdinPipe()
    if err != nil {
        return "", "", err
    }
    defer stdin.Close()

    fzf.Stderr = os.Stderr

    stdout, err := fzf.StdoutPipe()
    if err != nil {
        return "", "", err
    }
    defer stdout.Close()

    if err = fzf.Start(); err != nil {
        return "", "", err
    }

    _, err = io.WriteString(stdin, strings.Join(options, "\n") + "\n")
    if err != nil {
        return "", "", err
    }

    fzfResult, err := io.ReadAll(stdout)
    if err != nil {
        return "", "", err
    }

    if err = fzf.Wait(); err != nil {
        // fzf still returns a non-zero exit code when nothing selected, but need to ignore that so this can still read the entered query and create a generic session
        //return "", "", err
    } 

    lines := strings.Split(string(fzfResult), "\n")

    //fmt.Println("Split result:", lines, len(lines))
    
    if len(lines) == 2 {
        return "", lines[0], nil
    } else if len(lines) == 3 {
        return lines[1], lines[0], nil
    } else {
        return "", "", errors.New("Invalid result returned from system fzf executable")
    }
}
