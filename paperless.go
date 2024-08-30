package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

func SetupPaperless() {
	username := os.Getenv("PAPERLESS_USERNAME")
	password := os.Getenv("PAPERLESS_PASSWORD")
	if username == "" || password == "" {
		log.Fatal("PAPERLESS_USERNAME or PAPERLESS_PASSWORD is not set")
	}
}

func PostToPaperless(url, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("document", filepath.Base(filePath))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}
	err = writer.Close()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url+"/api/documents/post_document/", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Add Basic Authentication
	username := os.Getenv("PAPERLESS_USERNAME")
	password := os.Getenv("PAPERLESS_PASSWORD")
	auth := username + ":" + password
	basicAuth := base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Set("Authorization", "Basic "+basicAuth)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to post to paperless: %s", resp.Status)
	}

	return nil
}
