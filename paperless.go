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
	"strings"
)

func SetupPaperless() {
	for _, instance := range CurrentSettings().PaperlessInstances {
		if paperlessToken(instance.Host) != "" {
			continue
		}
		username, password := paperlessCredentials(instance.Host)
		if username == "" || password == "" {
			log.Printf("warning: no credentials configured for paperless instance %s (set them in the settings view, or via PAPERLESS_%s_TOKEN, PAPERLESS_%s_USERNAME/PASSWORD or PAPERLESS_USERNAME/PASSWORD)", instance.Host, instanceEnvKey(instance.Host), instanceEnvKey(instance.Host))
		}
	}
}

// paperlessToken resolves the API token for an instance: the one stored in
// settings wins, then the per-host PAPERLESS_<HOST_WITH_UNDERSCORES>_TOKEN env
// var. An empty result means fall back to basic auth.
func paperlessToken(host string) string {
	for _, instance := range CurrentSettings().PaperlessInstances {
		if instance.Host == host && instance.Token != "" {
			return instance.Token
		}
	}
	return os.Getenv(fmt.Sprintf("PAPERLESS_%s_TOKEN", instanceEnvKey(host)))
}

func instanceEnvKey(host string) string {
	return strings.ToUpper(strings.ReplaceAll(host, ".", "_"))
}

// paperlessCredentials resolves credentials for an instance: the ones stored
// in settings win, then per-host env vars
// (PAPERLESS_<HOST_WITH_UNDERSCORES>_USERNAME/PASSWORD), then the generic
// PAPERLESS_USERNAME/PASSWORD pair.
func paperlessCredentials(host string) (string, string) {
	for _, instance := range CurrentSettings().PaperlessInstances {
		if instance.Host == host && instance.Username != "" && instance.Password != "" {
			return instance.Username, instance.Password
		}
	}
	key := instanceEnvKey(host)
	username := os.Getenv(fmt.Sprintf("PAPERLESS_%s_USERNAME", key))
	password := os.Getenv(fmt.Sprintf("PAPERLESS_%s_PASSWORD", key))
	if username == "" || password == "" {
		username = os.Getenv("PAPERLESS_USERNAME")
		password = os.Getenv("PAPERLESS_PASSWORD")
	}
	return username, password
}

func PostToPaperless(instance, filePath string) error {
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

	req, err := http.NewRequest("POST", "https://"+instance+"/api/documents/post_document/", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	var username string
	if token := paperlessToken(instance); token != "" {
		req.Header.Set("Authorization", "Token "+token)
		username = "<token>"
	} else {
		var password string
		username, password = paperlessCredentials(instance)
		auth := username + ":" + password
		basicAuth := base64.StdEncoding.EncodeToString([]byte(auth))
		req.Header.Set("Authorization", "Basic "+basicAuth)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := strings.TrimSpace(string(detail))
		if msg == "" {
			msg = "(empty response body)"
		}
		if username == "" {
			msg = "no credentials configured; " + msg
		}
		if strings.Contains(msg, "MFA required") {
			msg += " (this account has MFA enabled, which paperless refuses over basic auth — set an API token for this instance instead)"
		}
		return fmt.Errorf("%s: %s: %s", instance, resp.Status, msg)
	}

	return nil
}
