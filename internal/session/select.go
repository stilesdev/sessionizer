package session

import (
	"errors"

	"github.com/adrg/xdg"
	"github.com/stilesdev/sessionizer/internal/fzf"
)

var AbortedError = errors.New("User aborted selection")

func SelectFromFzfPrompt(list []Session) (Session, error) {
	fzfEntries := make([]string, len(list))
	for idx, s := range list {
		fzfEntries[idx] = s.FzfEntry()
	}

	selectedIndex, _, enteredQuery, err := fzf.Prompt(fzfEntries)
	if err != nil {
		return Session{}, err
	}

	if selectedIndex < 0 && enteredQuery == "" {
		// user aborted without making a selection
		return Session{}, AbortedError
	}

	if selectedIndex < 0 {
		// no selection made but text was entered, create scratch session
		return Session{
			Name:       enteredQuery,
			Path:       xdg.Home,
			Exists:     false,
			IsAttached: false,
			IsScratch:  true,
		}, nil
	}

	// selection was made, use it
	return list[selectedIndex], nil
}

func SelectFromOverride(list []Session, override string) (Session, error) {
	for _, s := range list {
		if s.Name == override || s.Path == override {
			return s, nil
		}
	}

	return Session{}, errors.New("No session matches provided override")
}
