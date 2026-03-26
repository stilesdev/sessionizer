package config

import (
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
	"github.com/stilesdev/sessionizer/internal/log"
	"github.com/stilesdev/sessionizer/internal/tmux"
)

type Config struct {
	Tmux     Tmux
	Sessions []Session
}

type Tmux struct {
	HideAttachedSessions bool
}

type Session struct {
	Path    string
	Paths   []string
	Env     map[string]string
	Command string
	Split   tmux.PaneSplit
	Windows []tmux.TmuxWindow
}

func GetDefaultConfigFile() (string, error) {
	return xdg.ConfigFile("sessionizer/config.toml")
}

func Parse(configFile string) (config Config, err error) {
	defaultConfigFile, err := GetDefaultConfigFile()
	if err != nil {
		return config, err
	}

	if _, err := toml.DecodeFile(configFile, &config); err == nil {
		log.Debug("Using config file:", configFile)
		return config, nil
	}

	if configFile != defaultConfigFile {
		log.Debug("User specified config file but we couldn't decode it:", configFile)
		return config, err
	}

	log.Debug("Using default config, error loading file:", err)

	// not able to load default config, set defaults for anything required here:
	defaultSessionConfig := Session{
		Path: filepath.Join(xdg.Home, "*"),
	}
	config.Sessions = append(config.Sessions, defaultSessionConfig)

	return config, err
}
