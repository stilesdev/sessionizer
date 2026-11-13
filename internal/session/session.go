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

func newSessionFromConfig(conf config.Session, path string) Session {
	session := Session{
		Name:   filepath.Base(path),
		Path:   path,
		Config: conf.SessionConfig,
	}

	for key, val := range session.Config.Env {
		session.Config.Env[key] = expandHome(val)
	}

	if session.Config.Split.Path != "" {
		session.Config.Split.Path = expandHome(session.Config.Split.Path)
	}

	for key, val := range session.Config.Split.Env {
		session.Config.Split.Env[key] = expandHome(val)
	}

	for i, window := range session.Config.Windows {
		if window.Path != "" {
			session.Config.Windows[i].Path = expandHome(window.Path)
		}
		for key, val := range window.Env {
			session.Config.Windows[i].Env[key] = expandHome(val)
		}
	}

	return session
}

func BuildSessionList(conf config.Config, existingSessions []Session, sessionOverride string) []Session {
	sessions := parseConfigSessions(conf.Sessions)

	sessions = mergeExistingSessions(sessions, existingSessions)

	// sessionOverride bypasses the HideAttachedSessions config here so it only hides attached sessions when showing the fzf interface.
	if conf.Tmux.HideAttachedSessions && sessionOverride == "" {
		sessions = filterAttachedSessions(sessions)
	}

	sortSessions(&sessions)

	return sessions
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

func parseConfigSessions(configSessions []config.Session) (sessions []Session) {
	for _, sessionConfig := range configSessions {
		var allPaths []string
		if sessionConfig.Path != "" {
			allPaths = append(allPaths, sessionConfig.Path)
		}
		allPaths = append(allPaths, sessionConfig.Paths...)

		for _, glob := range allPaths {
			for _, path := range parseGlobToPaths(glob) {
				session := newSessionFromConfig(sessionConfig, path)

				// if multiple sessions match the same path, use the last one in the config so later sections override earlier ones
				if found, index := findSessionIndex(session, sessions); found {
					sessions[index] = session
				} else {
					sessions = append(sessions, session)
				}
			}
		}
	}

	return sessions
}

func mergeExistingSessions(configSessions []Session, existingSessions []Session) []Session {
	merged := configSessions

	for _, existing := range existingSessions {
		if found, index := findSessionIndex(existing, merged); found {
			merged[index].Exists = true
			merged[index].IsAttached = existing.IsAttached
		} else {
			existing.IsScratch = true
			merged = append(merged, existing)
		}
	}

	return merged
}

func filterAttachedSessions(sessions []Session) []Session {
	return slices.DeleteFunc(sessions, func(s Session) bool {
		return s.IsAttached
	})
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
