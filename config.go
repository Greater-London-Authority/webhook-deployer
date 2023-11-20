package main

import (
	"encoding/json"
	"log"
	"os"
)

type Config struct {
	Listen   string          `json:"listen"`
	Secret   string          `json:"secret"`
	GHToken  string          `json:"GH_TOKEN"`
	Projects []ProjectConfig `json:"projects"`
}

type ProjectConfig struct {
	Repository   string `json:"repository"`
	Destination  string `json:"destination"`
	WorkflowPath string `json:"workflow_path"`
	NtfyTopic    string `json:"ntfy_topic"`
}

func findFirstDuplicatedDestination(projects []ProjectConfig) string {
	set := make(map[string]bool)
	for _, project := range projects {
		if _, alreadyExists := set[project.Destination]; alreadyExists {
			return project.Destination
		} else {
			set[project.Destination] = true
		}
	}
	return ""
}

func readConfig(configPath string) Config {
	config := Config{}

	file, err := os.ReadFile(configPath)
	if err != nil {
		log.Panic("Failed to read config file ", configPath, ":", err)
	}

	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Panic(err)
	}

	if config.GHToken == "" {
		log.Panic("Config file doesn't include a GitHub token")
	}

	if duplicate := findFirstDuplicatedDestination(config.Projects); duplicate != "" {
		log.Panic("Error in config file - more than one project uses the destination: ", duplicate)
	}

	return config
}
