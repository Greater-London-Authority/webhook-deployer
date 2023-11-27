package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
)

type DeployLogEntry struct {
	// Project    string `json:"project"`
	Commit     string `json:"commit"`
	DeployedAt string `json:"deployed_at"`
}

type DeployLog struct {
	Projects map[string]DeployLogEntry `json:""`
	//	Entries []DeployLogEntry `json:"entries"`
}

var mu sync.Mutex

func updateDeploymentLog(deployLogPath string, project string, commit string, deployedAt string) {

	var deployLog DeployLog

	if (deployLogPath == "") || (project == "") {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// if file exists and is non-empty, load its contents
	if _, err := os.Stat(deployLogPath); !errors.Is(err, os.ErrNotExist) {

		file, err := os.ReadFile(deployLogPath)
		if err != nil {
			log.Panic("Deployment log file exists but could not be read", deployLogPath, ":", err)
		}

		if len(file) > 0 {
			err = json.Unmarshal(file, &deployLog)
			if err != nil {
				fmt.Println("Error parsing deployment log:", err)
				return
			}
		}
	}

	if deployLog.Projects == nil {
		deployLog.Projects = make(map[string]DeployLogEntry)
	}

	// update entry for this project
	deployLog.Projects[project] = DeployLogEntry{
		Commit:     commit,
		DeployedAt: deployedAt,
	}

	// write updated log to file
	jsonString, err := json.MarshalIndent(deployLog, "", "  ")
	if err != nil {
		log.Println("Failed to marshal deployment log:", err)
	}

	err = os.WriteFile(deployLogPath, jsonString, 0644)
	if err != nil {
		log.Println("Failed to write deployment log to file:", err)
	}

	log.Println("Updated deployment log for project", project, "with commit", commit, "deployed at", deployedAt)
}
