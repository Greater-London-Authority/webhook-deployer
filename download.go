package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func downloadFromURL(url string, token string, destination string) error {

	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println("Error constructing GET request for download:", err)
		return fmt.Errorf("constructing GET request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error making GET request for download:", err)
		return fmt.Errorf("downloading artifact: %w", err)
	}

	if resp.StatusCode != 200 {
		log.Println("Error - received status code", resp.StatusCode, "for URL", url)
		return errors.New("error downloading artifact")
	}

	defer resp.Body.Close()

	tmpDir, err := os.MkdirTemp("", "webhook-handler")
	if err != nil {
		log.Println("Error creating tmp dir to save zip file:", err)
		return fmt.Errorf("creating tmp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "file.zip")
	out, err := os.Create(zipPath)
	if err != nil {
		log.Println("Error creating file to save zip file to:", err)
		return fmt.Errorf("creating file to save zip file to: %w", err)
	}
	defer out.Close()

	// log.Println("Saving to:", zipPath)
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Println("Error saving downloaded file to tmp dir:", err)
		return fmt.Errorf("saving downloaded file to tmp dir: %w", err)
	}

	err = os.RemoveAll(destination)
	if err != nil {
		log.Println("Error removing existing files:", err)
		return fmt.Errorf("removing existing files: %w", err)
	}

	err = os.MkdirAll(destination, 0777)
	if err != nil {
		log.Println("Error creating destination dir:", err)
		return fmt.Errorf("creating destination dir: %w", err)
	}

	err = extractZipFile(zipPath, destination)
	if err != nil {
		log.Println("Error extracting zip file:", err)
		return fmt.Errorf("extracting zip file: %w", err)
	}

	return nil
}

func extractZipFile(zipPath string, destination string) error {
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		log.Println("Error opening zip file:", err)
		return fmt.Errorf("opening zip file: %w", err)
	}
	defer zipReader.Close()

	for _, file := range zipReader.File {
		// Sanitize path and ensure the sanitized path is within the destination directory
		sanitizedPath := filepath.Join(destination, filepath.Clean(file.Name))
		if !strings.HasPrefix(sanitizedPath, filepath.Clean(destination)+string(filepath.Separator)) {
			msg := fmt.Sprintf("Invalid file path: %s", file.Name)
			log.Print(msg)
			return errors.New(msg)
		}
		path := sanitizedPath

		if file.FileInfo().IsDir() {
			// don't need to do anything: directory will be created when we extract files
			continue
		}

		err = os.MkdirAll(filepath.Dir(path), os.ModePerm)
		if err != nil {
			log.Println("Error creating subdir to contain extracted file:", err)
			return fmt.Errorf("creating subdir to contain extracted file: %w", err)
		}

		zippedFile, err := file.Open()
		if err != nil {
			log.Println("Error extracting file from zip:", err)
			return fmt.Errorf("extracting file from zip: %w", err)
		}

		// N.B. we use these nested anonymous function to ensure files are closed on each loop iteration
		// (rather than waiting until extractZipFile exits), avoiding exhausing file descriptors.
		var extractionError error
		func() {
			defer zippedFile.Close()

			extractedFile, err := os.Create(path)
			if err != nil {
				log.Println("Error creating file to contain contents extracted from zip:", err)
				extractionError = fmt.Errorf("creating file to contain contents extracted from zip: %w", err)
				return
			}

			func() {
				defer extractedFile.Close()

				_, err = io.Copy(extractedFile, zippedFile)
				if err != nil {
					log.Println("Error saving file extracted from zip:", err)
					extractionError = fmt.Errorf("saving file extracted from zip: %w", err)
				}
			}()
		}()

		if extractionError != nil {
			return extractionError
		}
	}

	return nil
}
