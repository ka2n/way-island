package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const userStyleCSSRelativePath = "way-island/style.css"
const userConfigRelativePath = "way-island/config.json"

type appConfig struct {
	Focus focusConfig `json:"focus"`
}

type focusConfig struct {
	RetitleWithOSC bool `json:"retitle_with_osc"`
}

func loadAppCSS(defaultCSS string) (string, error) {
	path, err := resolveUserStyleCSSPath()
	if err != nil {
		return defaultCSS, err
	}

	return loadAppCSSFromPath(defaultCSS, path)
}

func loadAppCSSFromPath(defaultCSS, path string) (string, error) {
	userCSS, err := loadUserStyleCSS(path)
	if err != nil {
		return defaultCSS, err
	}

	return mergeAppCSS(defaultCSS, userCSS), nil
}

func loadUserStyleCSS(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}

	return string(data), nil
}

func mergeAppCSS(defaultCSS, userCSS string) string {
	if userCSS == "" {
		return defaultCSS
	}

	return defaultCSS + "\n\n" + userCSS
}

func resolveUserStyleCSSPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, userStyleCSSRelativePath), nil
}

func loadAppConfig() (appConfig, error) {
	path, err := resolveUserConfigPath()
	if err != nil {
		return appConfig{}, err
	}

	return loadAppConfigFromPath(path)
}

func loadAppConfigFromPath(path string) (appConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return appConfig{}, nil
		}
		return appConfig{}, err
	}

	var cfg appConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return appConfig{}, err
	}

	return cfg, nil
}

func resolveUserConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, userConfigRelativePath), nil
}
