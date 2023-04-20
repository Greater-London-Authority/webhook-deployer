package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

// request for artifact
type Artifact struct {
	URL                string `json:"url"`
	ArchiveDownloadURL string `json:"archive_download_url"`
	Expired            bool   `json:"expired"`
}

type Response struct {
	TotalCount int        `json:"total_count"`
	Artifacts  []Artifact `json:"artifacts"`
}

func getDownloadURL(url string, token string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println("Error constructing GET request:", err)
		return "", errors.New("Error constructing GET request")
	}

	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making GET request:", err)
		return "", errors.New("Error making GET request")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body from API:", err)
		return "", errors.New("Error reading response body from API")
	}

	var data Response
	err = json.Unmarshal([]byte(body), &data)
	if err != nil {
		fmt.Println("Failed to parse JSON returned from API:", err)
		return "", errors.New("Failed to parse JSON returned from API")
	}

	// fmt.Println(string(body))

	if data.TotalCount != 1 {
		fmt.Println("Total count of artifacts is not 1, so ignoring")
		return "", errors.New("Total count of artifacts is not 1, so ignoring")
	}

	return data.Artifacts[0].ArchiveDownloadURL, nil
}
