package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const configFileName = ".relix.conf"

// getConfigPath returns the path to the config file
func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configFileName), nil
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
			return &AppConfig{}, nil
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
