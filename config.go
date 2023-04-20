package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
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
}

func readConfig(configPath string) Config {
	config := Config{}

	file, err := ioutil.ReadFile(configPath)
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

	return config
}
