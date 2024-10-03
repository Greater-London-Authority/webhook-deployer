package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type Repo struct {
	FullName string `json:"full_name"`
}

type WorkflowRun struct {
	ArtifactsURL string `json:"artifacts_url"`
	RunURL       string `json:"html_url"`
	HeadCommit   struct {
		ID string `json:"id"`
	} `json:"head_commit"`
	HeadBranch string `json:"head_branch"`
	CreatedAt  string `json:"created_at"`
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

type DeleteData struct {
	Repository Repo   `json:"repository"`
	RefType    string `json:"ref_type"` // branch or tag
	Ref        string `json:"ref"`      // branch or tag name
}

func getHandler(config Config) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			log.Printf("Request for path other than / or /health (%s)\n", r.URL.Path)
			http.NotFound(w, r)
			return
		}

		event := r.Header.Get("X-GitHub-Event")
		if event != "workflow_run" && event != "delete" {
			w.WriteHeader(http.StatusOK)
			log.Println("X-GitHub-Event is not workflow_run or delete, so ignoring")
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

		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("Cannot read the request body")
			return
		}

		if event == "delete" {
			var deleteData DeleteData

			err = json.Unmarshal(body, &deleteData)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Println("Cannot parse the request body")
				return
			}

			log.Println("Received delete event for", deleteData.RefType, deleteData.Ref, "in repo", deleteData.Repository.FullName)

			if deleteData.RefType != "branch" {
				log.Println("Ignoring delete event for", deleteData.RefType, deleteData.Ref, "in repo", deleteData.Repository.FullName, "as not for a branch")
				w.WriteHeader(http.StatusOK)
				return
			}

			var destination = ""
			for _, project := range config.Projects {
				if project.Repository == deleteData.Repository.FullName {
					if project.AllowBranchPreviews && deleteData.Ref != "master" && deleteData.Ref != "main" {
						destination = project.Destination + "-" + deleteData.Ref
					}

					break
				}
			}

			if destination == "" {
				log.Println("Not deleting, as no destination defined for repository", deleteData.Repository.FullName)
				w.WriteHeader(http.StatusOK)
				return
			}

			err = os.RemoveAll(destination)
			if err != nil {
				log.Println("Error deleting", destination, ":", err)
				return
			}

			return
		}

		if event == "workflow_run" {
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
			var ntfy_topics []string

			for _, project := range config.Projects {
				if project.Repository == data.Repository.FullName && project.WorkflowPath == data.Workflow.WorkflowPath {
					destination = project.Destination

					if project.AllowBranchPreviews && data.WorkflowRun.HeadBranch != "master" && data.WorkflowRun.HeadBranch != "main" {
						destination = destination + "-" + data.WorkflowRun.HeadBranch
					}

					if len(project.NtfyTopics) > 0 {
						ntfy_topics = project.NtfyTopics
					} else if len(project.NtfyTopic) > 0 {
						ntfy_topics = []string{project.NtfyTopic}
					}
					break
				}
			}

			if destination == "" {
				w.WriteHeader(http.StatusOK)
				log.Println("No action defined to match workflow", data.Workflow.WorkflowPath, "in repo", data.Repository.FullName)
			} else {
				log.Println("Handling workflow", data.Workflow.WorkflowPath, "in repo", data.Repository.FullName)

				downloadURL, err := getDownloadURL(data.WorkflowRun.ArtifactsURL, config.GHToken)

				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					log.Println("Cannot get download URL")
					return
				}

				// log the deployment
				updateDeploymentLog(config.DeployLogPath, data.Repository.FullName, data.WorkflowRun.HeadCommit.ID, data.WorkflowRun.CreatedAt)

				err = downloadFromURL(downloadURL, config.GHToken, destination)
				if err == nil {
					log.Println("Handled workflow", data.Workflow.WorkflowPath, "in repo", data.Repository.FullName, "and extracted to", destination)
					w.WriteHeader(http.StatusOK)
					for _, ntfy_topic := range ntfy_topics {
						sendMsg(ntfy_topic, fmt.Sprintf("Handled workflow %s in repo %s and extracted to %s", data.Workflow.WorkflowPath, data.Repository.FullName, destination), data.WorkflowRun.RunURL)
					}
					return
				} else {
					w.WriteHeader(http.StatusInternalServerError)
					return
					// TODO: pass error message
				}

			}
		}
	}

}

func sendMsg(topic string, msg string, url string) {
	req, _ := http.NewRequest("POST", "https://ntfy.sh/"+topic, strings.NewReader(msg))
	req.Header.Set("Title", "Successful deployment")
	req.Header.Set("Priority", "high")
	req.Header.Set("Tags", "rocket")
	req.Header.Set("Actions", fmt.Sprintf("view, View Workflow Run, %s, clear=true", url))
	_, err := http.DefaultClient.Do(req)

	if err != nil {
		log.Println("Error sending message to ntfy.sh:", err)
	}
}

func healthcheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		log.Println("Error writing healthcheck response")
	}
}

func main() {
	var configPath string

	if len(os.Args) >= 2 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		log.Println("Usage: webhook-handler <config file>")
		os.Exit(0)
	} else if len(os.Args) >= 2 && os.Args != nil {
		log.Println("Using config path:", os.Args[1])
		configPath = os.Args[1]
	} else {
		configPath = "config.json"
		log.Println("Using default config path (config.json)")
	}
	config := readConfig(configPath)

	http.HandleFunc("/health", healthcheck)

	http.HandleFunc("/", getHandler(config))

	if len(config.Listen) > 0 {
		log.Fatal(http.ListenAndServe(config.Listen, nil))
	} else {
		log.Fatal(http.ListenAndServe(":8080", nil))
	}
}
