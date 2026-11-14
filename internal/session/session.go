package session

import (
	"fmt"
	"path/filepath"

	"github.com/stilesdev/sessionizer/internal/config"
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
