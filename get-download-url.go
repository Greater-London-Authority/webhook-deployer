package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
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

func getDownloadData(url string, token string) (Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println("Error constructing GET request:", err)
		return Response{}, errors.New("Error constructing GET request")
	}

	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error making GET request:", err)
		return Response{}, errors.New("Error making GET request")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading response body from API:", err)
		return Response{}, errors.New("Error reading response body from API")
	}

	var data Response
	err = json.Unmarshal([]byte(body), &data)
	if err != nil {
		log.Println("Failed to parse JSON returned from API:", err)
		return Response{}, errors.New("Failed to parse JSON returned from API")
	}

	return data, nil
}

func getDownloadURL(url string, token string) (string, error) {
	data, err := getDownloadData(url, token)
	if err != nil {
		return "", err
	}

	if data.TotalCount == 0 {
		log.Println("Total count of artifacts is not 0, so re-fetching after 5 seconds")
		time.Sleep(5 * time.Second)

		data, err = getDownloadData(url, token)
		if err != nil {
			return "", err
		}
	}

	if data.TotalCount != 1 {
		log.Println(fmt.Sprintf("Total count of artifacts is %d not 1, so ignoring", data.TotalCount))
		return "", errors.New("Total count of artifacts is not 1, so ignoring")
	}

	return data.Artifacts[0].ArchiveDownloadURL, nil
}
