package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Odoo uploads go through the JSON-2 API of an Odoo whose documents live in a
// paperless-ngx archive behind it (usl.document.upload_from_odoo). Odoo, not
// the scanner, owns the legal company and the confidentiality of the document,
// so every upload names one company. The archive's OCR runs after the call
// returns; the scanner does not wait for it.

const odooUserAgent = "escl-scanner-2-merger/1.0"

var odooHTTP = &http.Client{Timeout: 3 * time.Minute}

// odooAPIKey resolves the API key: the one stored in settings wins, then the
// ODOO_API_KEY environment variable.
func odooAPIKey() string {
	if key := CurrentSettings().Odoo.APIKey; key != "" {
		return key
	}
	return os.Getenv("ODOO_API_KEY")
}

// odooCall posts one JSON-2 request and decodes the JSON answer into out.
func odooCall(model, method string, body map[string]any, out any) error {
	cfg := CurrentSettings().Odoo
	if cfg.URL == "" {
		return fmt.Errorf("odoo: no URL configured")
	}
	key := odooAPIKey()
	if key == "" {
		return fmt.Errorf("odoo: no API key configured (set it in the settings view or ODOO_API_KEY)")
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", "https://"+cfg.URL+"/json/2/"+model+"/"+method, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "bearer "+key)
	req.Header.Set("User-Agent", odooUserAgent)
	if cfg.Database != "" {
		req.Header.Set("X-Odoo-Database", cfg.Database)
	}
	resp, err := odooHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("odoo: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("odoo: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("odoo %s.%s: %s: %s", model, method, resp.Status, odooErrorMessage(data))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

// odooErrorMessage extracts the human message from a JSON-2 error body.
func odooErrorMessage(data []byte) string {
	var e struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &e) == nil && e.Message != "" {
		return e.Message
	}
	msg := strings.TrimSpace(string(data))
	if len(msg) > 512 {
		msg = msg[:512]
	}
	if msg == "" {
		msg = "(empty response body)"
	}
	return msg
}

// FetchOdooCompanies lists the companies the API key may use, so the settings
// view can offer them without typing ids by hand.
func FetchOdooCompanies() ([]OdooCompany, error) {
	var rows []OdooCompany
	err := odooCall("res.company", "search_read", map[string]any{
		"domain": []any{},
		"fields": []string{"id", "name"},
		"order":  "name",
	}, &rows)
	return rows, err
}

func IsConfiguredOdooCompany(id int) bool {
	for _, company := range CurrentSettings().Odoo.Companies {
		if company.ID == id {
			return true
		}
	}
	return false
}

func odooCompanyName(id int) string {
	for _, company := range CurrentSettings().Odoo.Companies {
		if company.ID == id {
			return company.Name
		}
	}
	return fmt.Sprintf("company %d", id)
}

// PostToOdoo archives one scan in Odoo for one company.
func PostToOdoo(companyID int, filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	cfg := CurrentSettings().Odoo
	confidentiality := cfg.Confidentiality
	if confidentiality == "" {
		confidentiality = "internal"
	}
	var result map[string]any
	err = odooCall("usl.document", "upload_from_odoo", map[string]any{
		"filename":        time.Now().Local().Format("2006.01.02-15.04.05") + "-scan.pdf",
		"content_base64":  base64.StdEncoding.EncodeToString(content),
		"content_type":    "application/pdf",
		"company_id":      companyID,
		"confidentiality": confidentiality,
		"context":         map[string]any{"allowed_company_ids": []int{companyID}},
	}, &result)
	if err != nil {
		return fmt.Errorf("%s: %w", odooCompanyName(companyID), err)
	}
	return nil
}
