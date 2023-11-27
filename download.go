package main

import (
	"archive/zip"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func downloadFromURL(url string, token string, destination string) error {

	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println("Error constructing GET request for download:", err)
		return errors.New("Error constructing GET request for download")
	}

	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error constructing GET request for download:", err)
		return errors.New("Error downlaoding artifact")
	}
	defer resp.Body.Close()

	tmpDir, err := os.MkdirTemp("", "webhook-handler")
	if err != nil {
		log.Println("Error creating tmp dir to save zip file:", err)
		return errors.New("Error creating tmp dir to save zip file")
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "file.zip")
	out, err := os.Create(zipPath)
	if err != nil {
		log.Println("Error creating file to save zip file to:", err)
		return errors.New("Error creating file to save zip file to")
	}
	defer out.Close()

	// log.Println("Saving to:", zipPath)
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Println("Error saving downlaoded file to tmp dir:", err)
		return errors.New("Error saving downlaoded file to tmp dir")
	}

	err = os.RemoveAll(destination)
	if err != nil {
		log.Println("Error removing existing files:", err)
		return errors.New("Error removing existing files")
	}

	err = os.MkdirAll(destination, 0777)
	if err != nil {
		log.Println("Error creating destination dir:", err)
		return errors.New("Error creating destination dir")
	}

	err = extractZipFile(zipPath, destination)
	if err != nil {
		log.Println("Error extracting zip file:", err)
		return errors.New("Error extracting zip file")
	}

	return nil
}

func extractZipFile(zipPath string, destination string) error {
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		log.Println("Error opening zip file:", err)
		return errors.New("Error opening zip file")
	}
	defer zipReader.Close()

	for _, file := range zipReader.File {
		//
		path := filepath.Join(destination, file.Name)
		if file.FileInfo().IsDir() {
			// don't need to do anything: directory will be created when we extract files
			continue
		}

		err = os.MkdirAll(filepath.Dir(path), os.ModePerm)
		if err != nil {
			log.Println("Error creating subdir to contain extracted file:", err)
			return errors.New("Error creating subdir to contain extracted file")
		}

		zippedFile, err := file.Open()
		if err != nil {
			log.Println("Error extracting file from zip:", err)
			return errors.New("Error extracting file from zip")
		}
		defer zippedFile.Close()

		extractedFile, err := os.Create(filepath.Join(destination, file.Name))
		if err != nil {
			log.Println("Error creating file to contain contents extracted from zip:", err)
			return errors.New("Error creating file to contain contents extracted from zip")
		}
		defer extractedFile.Close()

		_, err = io.Copy(extractedFile, zippedFile)
		if err != nil {
			log.Println("Error saving file extracted from zip:", err)
			return errors.New("Error saving file extracted from zip")
		}

		// fmt.Printf("Extracted %s\n", file.Name)
	}
	return nil
}
