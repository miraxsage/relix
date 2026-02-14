package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	configDir       = ".relix"
	configFileName  = "config.json"
	releaseFileName = "release.json"
)

// getConfigDir returns the path to the config directory, creating it if needed
func getConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, configDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// getConfigPath returns the path to the config file
func getConfigPath() (string, error) {
	dir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// getReleaseStatePath returns the path to the release state file
func getReleaseStatePath() (string, error) {
	dir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, releaseFileName), nil
}

// LoadConfig loads the application configuration from file
func LoadConfig() (*AppConfig, error) {
	path, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config with default excluded file patterns and default theme
			return &AppConfig{
				ExcludePatterns: ".gitlab-ci.yml\nsprite.gen.ts",
				SelectedTheme:   "indigo",
				Themes: []ThemeConfig{
					{
						Name:       "indigo",
						Accent:     "#5F5FDF",
						Foreground: "#D7D7FF",
						Notion:     "#5F5F8A",
						Success:    "#00D588",
						Warning:    "#FFD600",
						Error:      "#FF84A8",
					},
				},
			}, nil
		}
		return nil, err
	}

	var config AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveConfig saves the application configuration to file
func SaveConfig(config *AppConfig) error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// SaveSelectedProject saves the selected project to config
func SaveSelectedProject(project *Project) error {
	config, err := LoadConfig()
	if err != nil {
		config = &AppConfig{}
	}

	if project == nil {
		config.SelectedProjectID = 0
		config.SelectedProjectPath = ""
		config.SelectedProjectName = ""
		config.SelectedProjectShortName = ""
	} else {
		config.SelectedProjectID = project.ID
		config.SelectedProjectPath = project.PathWithNamespace
		config.SelectedProjectName = project.NameWithNamespace
		config.SelectedProjectShortName = project.Name
	}

	return SaveConfig(config)
}

// LoadReleaseState loads the release state from file
func LoadReleaseState() (*ReleaseState, error) {
	path, err := getReleaseStatePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No release in progress
		}
		return nil, err
	}

	var state ReleaseState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// SaveReleaseState saves the release state to file
func SaveReleaseState(state *ReleaseState) error {
	path, err := getReleaseStatePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ClearReleaseState removes the release state file
func ClearReleaseState() error {
	path, err := getReleaseStatePath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil // Already cleared
	}
	return err
}
