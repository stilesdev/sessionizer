package session

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/adrg/xdg"
	"github.com/stilesdev/sessionizer/internal/config"
	"github.com/stilesdev/sessionizer/internal/log"
	"github.com/stilesdev/sessionizer/internal/tmux"
)

type Session struct {
	Path       string
	Name       string
	FzfEntry   string
	Exists     bool
	IsScratch  bool
	IsAttached bool
	Env        map[string]string
	Command    string
	Split      tmux.PaneSplit
	Windows    []tmux.TmuxWindow
}

func NewSessionList(existingTmuxSessions []tmux.TmuxSession, conf config.Config, sessionOverride string) (sessions []Session, err error) {
	for _, sessionConfig := range conf.Sessions {
		if sessionConfig.Path != "" {
			for _, path := range parseGlobToPaths(sessionConfig.Path) {
				session := parseSession(path, sessionConfig, existingTmuxSessions)
				if !session.IsAttached || !conf.Tmux.HideAttachedSessions || sessionOverride != "" {
					if found, index := findSessionIndex(session, sessions); found {
						sessions[index] = session
					} else {
						sessions = append(sessions, session)
					}
				}
			}
		}

		if len(sessionConfig.Paths) > 0 {
			for _, glob := range sessionConfig.Paths {
				for _, path := range parseGlobToPaths(glob) {
					session := parseSession(path, sessionConfig, existingTmuxSessions)
					if !session.IsAttached || !conf.Tmux.HideAttachedSessions || sessionOverride != "" {
						if found, index := findSessionIndex(session, sessions); found {
							sessions[index] = session
						} else {
							sessions = append(sessions, session)
						}
					}
				}
			}
		}
	}

	// look for any scratch sessions - existing sessions in tmux but not associated with any paths from config file
	for _, existingSession := range existingTmuxSessions {
		excludeSession := false

		// exclude if attached and configured to hide attached sessions
		if (conf.Tmux.HideAttachedSessions && existingSession.Attached) || sessionOverride != "" {
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
				Path:       existingSession.Path,
				Name:       existingSession.Name,
				FzfEntry:   fmt.Sprintf("scratch: %s", existingSession.Name),
				Exists:     true,
				IsAttached: existingSession.Attached,
				IsScratch:  true,
			})
		}
	}

	sortSessions(&sessions)

	return sessions, nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~"+string(os.PathSeparator)) {
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

func parseGlobToPaths(glob string) []string {
	var paths []string

	matches, err := filepath.Glob(expandHome(glob))
	if err != nil {
		log.Debug("Unable to parse glob:", glob)
		return paths
	}

	for _, path := range matches {
		if fileInfo, err := os.Stat(path); err == nil && fileInfo.IsDir() {
			paths = append(paths, path)
		}
	}

	return paths
}

func findSessionIndex(target Session, sessions []Session) (bool, int) {
	for index, session := range sessions {
		if session.Path == target.Path {
			return true, index
		}
	}

	return false, 0
}

func parseSession(path string, sessionConfig config.Session, existingTmuxSessions []tmux.TmuxSession) Session {
	session := Session{
		Path:     path,
		Name:     filepath.Base(path),
		FzfEntry: unexpandHome(path),
		Env:      sessionConfig.Env,
		Command:  sessionConfig.Command,
		Split:    sessionConfig.Split,
		Windows:  sessionConfig.Windows,
	}

	for key, val := range session.Env {
		session.Env[key] = expandHome(val)
	}

	for _, tmuxSession := range existingTmuxSessions {
		if tmuxSession.Path == path && tmuxSession.Name == session.Name {
			session.FzfEntry = fmt.Sprintf("tmux: %s [%s]", session.Name, unexpandHome(path))
			session.Exists = true
			session.IsAttached = tmuxSession.Attached
			break
		}
	}

	if session.Split.Path != "" {
		session.Split.Path = expandHome(session.Split.Path)
	}

	for i, window := range session.Windows {
		if window.Path != "" {
			session.Windows[i].Path = expandHome(window.Path)
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
