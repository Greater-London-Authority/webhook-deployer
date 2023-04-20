package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

type Repo struct {
	FullName string `json:"full_name"`
}

type WorkflowRun struct {
	ArtifactsURL string `json:"artifacts_url"`
}

type Workflow struct {
	WorkflowPath string `json:"path"`
}

type Data struct {
	Action      string      `json:"action"`
	Repository  Repo        `json:"repository"`
	WorkflowRun WorkflowRun `json:"workflow_run"`
	Workflow    Workflow    `json:"workflow"`
}

func getHandler(config Config) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		event := r.Header.Get("X-GitHub-Event")
		if event != "workflow_run" {
			w.WriteHeader(http.StatusOK)
			log.Println("X-GitHub-Event is not workflow_run, so ignoring")
			return
		}

		secret := os.Getenv("GITHUB_SECRET")
		if secret != "" {
			if !isValidSignature(r, secret) {
				w.WriteHeader(http.StatusUnauthorized)
				log.Println("X-Hub-Signature is not correct, so ignoring")
				return
			}
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("Cannot read the request body")
			return
		}

		var data Data
		err = json.Unmarshal(body, &data)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("Cannot parse the request body")
			return
		}

		if data.Action != "completed" {
			w.WriteHeader(http.StatusOK)
			log.Println("Action is not completed, so ignoring")
			return
		}

		var destination = ""
		for _, project := range config.Projects {
			if project.Repository == data.Repository.FullName && project.WorkflowPath == data.Workflow.WorkflowPath {
				destination = project.Destination
				break
			}
		}

		if destination == "" {
			w.WriteHeader(http.StatusOK)
			log.Println("No action defined to match workflow", data.Workflow.WorkflowPath, " in repo ", data.Repository.FullName)
		} else {
			log.Println("Handling workflow", data.Workflow.WorkflowPath, " in repo ", data.Repository.FullName)

			downloadURL, err := GetDownloadURL(data.WorkflowRun.ArtifactsURL, config.GHToken)

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Println("Cannot get download URL")
				return
			}

			err = DownloadFromURL(downloadURL, config.GHToken, destination)
			if err == nil {
				log.Println("Handled workflow", data.Workflow.WorkflowPath, " in repo ", data.Repository.FullName, " and extracted to ", destination)
				w.WriteHeader(http.StatusOK)
				return
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				return
				// TODO: pass error message
			}

		}
	}

}

func main() {
	var configPath string

	if len(os.Args) >= 2 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		fmt.Println("Usage: webhook-handler <config file>")
		os.Exit(0)
	} else if len(os.Args) >= 2 && os.Args != nil {
		fmt.Println(os.Args[1])
		configPath = os.Args[1]
	} else {
		configPath = "config.json"
		log.Println("Using default config path (config.json)")
	}
	config := ReadConfig(configPath)

	http.HandleFunc("/", getHandler(config))

	if len(config.Listen) > 0 {
		log.Fatal(http.ListenAndServe(config.Listen, nil))
	} else {
		log.Fatal(http.ListenAndServe(":8080", nil))
	}
}
