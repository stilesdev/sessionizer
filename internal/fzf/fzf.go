package fzf

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func IsAvailable() bool {
	if _, err := exec.LookPath("fzf"); err != nil {
		return false
	}

	return true
}

func addIndexes(options []string) []string {
	indexed := make([]string, len(options))

	for i := 0; i < len(options); i++ {
		indexed[i] = fmt.Sprintf("%d %s", i, options[i])
	}

	return indexed
}

func stripIndex(line string) (int, string, error) {
	index, entry, found := strings.Cut(line, " ")

	if !found {
		return 0, index, errors.New("Unable to parse index from selected line")
	}

	indexInt, err := strconv.Atoi(index)
	if err != nil {
		return -1, "", err
	}

	return indexInt, entry, nil
}

func promptError(err error) (int, string, string, error) {
	return -1, "", "", err
}

// Prompt will call the system fzf executable to prompt for the user to pick an option from the provided options.
//
// Except in the case of an error or the user exiting fzf with Esc or Ctrl-C, enteredQuery will always contain the text the user entered
// to filter the list of options, even the query does not match any of the provided options.
//
// If the user exits fzf without making a selection, this returns -1, "", "", nil
func Prompt(options []string) (selectedIndex int, selectedOption string, enteredQuery string, err error) {
	fzf := exec.Command("fzf", "--exact", "--print-query", "--no-sort", "--tac", "--cycle", "--with-nth", "2..", "--bind", "tab:print-query")
	stdin, err := fzf.StdinPipe()
	if err != nil {
		return promptError(err)
	}
	defer stdin.Close()

	fzf.Stderr = os.Stderr

	stdout, err := fzf.StdoutPipe()
	if err != nil {
		return promptError(err)
	}
	defer stdout.Close()

	if err = fzf.Start(); err != nil {
		return promptError(err)
	}

	_, err = io.WriteString(stdin, strings.Join(addIndexes(options), "\n")+"\n")
	if err != nil {
		return promptError(err)
	}

	fzfResult, err := io.ReadAll(stdout)
	if err != nil {
		return promptError(err)
	}

	if err = fzf.Wait(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			// 0 = normal
			// 1 = no match
			// 2 = error
			// 130 = user exited (Esc or Ctrl-C)
			if exitError.ExitCode() == 130 {
				return promptError(nil)
			} else if exitError.ExitCode() != 0 && exitError.ExitCode() != 1 {
				return promptError(err)
			}
		} else {
			return promptError(err)
		}
	}

	lines := strings.Split(string(fzfResult), "\n")

	if len(lines) == 2 {
		// an item was not selected from the list, return just the query entered into fzf
		return -1, "", lines[0], nil
	} else if len(lines) == 3 {
		// an item was selected from the list, return the selected index and item selected in addition to the query entered into fzf
		idx, entry, err := stripIndex(lines[1])
		if err != nil {
			return promptError(err)
		}
		return idx, entry, lines[0], nil
	} else {
		return promptError(errors.New("Invalid result returned from system fzf executable"))
	}
}
