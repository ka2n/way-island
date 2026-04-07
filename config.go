package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const userStyleCSSRelativePath = "way-island/style.css"
const userThemeCSSRelativePath = "way-island/user_style.css"
const userConfigRelativePath = "way-island/config.json"

type appConfig struct {
	Focus focusConfig `json:"focus"`
}

type focusConfig struct {
	TmuxSetTitles bool `json:"tmux_set_titles"`
}

func loadAppCSS(defaultCSS string) (string, error) {
	paths, err := resolveUserCSSPaths()
	if err != nil {
		return defaultCSS, err
	}

	return loadAppCSSFromPaths(defaultCSS, paths.StylePath, paths.UserStylePath)
}

type userCSSPaths struct {
	StylePath     string
	UserStylePath string
}

func loadAppCSSFromPaths(defaultCSS, stylePath, userStylePath string) (string, error) {
	styleCSS, styleExists, err := loadOptionalCSS(stylePath)
	if err != nil {
		return defaultCSS, err
	}

	baseCSS := defaultCSS
	if styleExists {
		baseCSS = styleCSS
	}

	userCSS, err := loadUserStyleCSS(userStylePath)
	if err != nil {
		return baseCSS, err
	}

	return mergeAppCSS(baseCSS, userCSS), nil
}

func loadUserStyleCSS(path string) (string, error) {
	data, _, err := loadOptionalCSS(path)
	return data, err
}

func loadOptionalCSS(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}

	return string(data), true, nil
}

func mergeAppCSS(defaultCSS, userCSS string) string {
	if userCSS == "" {
		return defaultCSS
	}

	return strings.TrimRight(defaultCSS, "\n") + "\n\n" + userCSS
}

func resolveUserStyleCSSPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, userStyleCSSRelativePath), nil
}

func resolveUserThemeCSSPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, userThemeCSSRelativePath), nil
}

func resolveUserCSSPaths() (userCSSPaths, error) {
	stylePath, err := resolveUserStyleCSSPath()
	if err != nil {
		return userCSSPaths{}, err
	}

	userStylePath, err := resolveUserThemeCSSPath()
	if err != nil {
		return userCSSPaths{}, err
	}

	return userCSSPaths{
		StylePath:     stylePath,
		UserStylePath: userStylePath,
	}, nil
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
