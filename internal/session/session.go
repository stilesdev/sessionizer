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
)

type Session struct {
	Name       string
	Path       string
	Exists     bool
	IsScratch  bool
	IsAttached bool
	Config     config.SessionConfig
}

func (s Session) FzfEntry() string {
	if s.IsScratch {
		// if it is a scratch session, it must already exist by definition
		return fmt.Sprintf("scratch: %s", s.Name)
	}

	path := unexpandHome(s.Path)

	if s.Exists {
		return fmt.Sprintf("tmux: %s [%s]", s.Name, path)
	}

	return path
}

func MergeSessionLists(existingSessions []Session, conf config.Config, sessionOverride string) (sessions []Session, err error) {
	for _, sessionConfig := range conf.Sessions {
		var allPaths []string
		if sessionConfig.Path != "" {
			allPaths = append(allPaths, sessionConfig.Path)
		}
		allPaths = append(allPaths, sessionConfig.Paths...)

		for _, glob := range allPaths {
			for _, path := range parseGlobToPaths(glob) {
				session := parseSession(path, sessionConfig, existingSessions)
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

	// look for any scratch sessions - existing sessions in tmux but not associated with any paths from config file
	for _, existingSession := range existingSessions {
		excludeSession := false

		// exclude if attached and configured to hide attached sessions
		if (conf.Tmux.HideAttachedSessions && existingSession.IsAttached) || sessionOverride != "" {
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
				Name:       existingSession.Name,
				Path:       existingSession.Path,
				Exists:     true,
				IsAttached: existingSession.IsAttached,
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

func parseSession(path string, sessionConfig config.Session, existingSessions []Session) Session {
	session := Session{
		Name:   filepath.Base(path),
		Path:   path,
		Config: sessionConfig.SessionConfig,
	}

	for key, val := range session.Config.Env {
		session.Config.Env[key] = expandHome(val)
	}

	for _, existingSession := range existingSessions {
		if existingSession.Path == path && existingSession.Name == session.Name {
			session.Exists = true
			session.IsAttached = existingSession.IsAttached
			break
		}
	}

	if session.Config.Split.Path != "" {
		session.Config.Split.Path = expandHome(session.Config.Split.Path)
	}

	for i, window := range session.Config.Windows {
		if window.Path != "" {
			session.Config.Windows[i].Path = expandHome(window.Path)
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
