package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withOdoo points the settings at a fake Odoo for the duration of a test.
func withOdoo(t *testing.T, handler http.HandlerFunc, companies []OdooCompany) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	previousClient, previousSettings := odooHTTP, CurrentSettings()
	odooHTTP = server.Client()
	settingsMux.Lock()
	appSettings.Odoo = OdooConfig{
		URL:       strings.TrimPrefix(server.URL, "https://"),
		APIKey:    "test-key",
		Companies: companies,
	}
	settingsMux.Unlock()
	t.Cleanup(func() {
		odooHTTP = previousClient
		settingsMux.Lock()
		appSettings = previousSettings
		settingsMux.Unlock()
	})
}

func TestPostToOdooSendsTheCompanyAndTheFile(t *testing.T) {
	pdf := []byte("%PDF-1.4 test scan")
	dir := t.TempDir()
	file := filepath.Join(dir, "scanned.pdf")
	if err := os.WriteFile(file, pdf, 0o600); err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	withOdoo(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/2/usl.document/upload_from_odoo" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "bearer test-key" {
			t.Errorf("authorization header = %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(`{"state": "processing", "operation_id": 7}`))
	}, []OdooCompany{{ID: 4, Name: "RW110"}})

	if err := PostToOdoo(4, file); err != nil {
		t.Fatalf("PostToOdoo: %v", err)
	}
	if got["company_id"] != float64(4) {
		t.Errorf("company_id = %v", got["company_id"])
	}
	if got["confidentiality"] != "internal" {
		t.Errorf("confidentiality = %v", got["confidentiality"])
	}
	decoded, _ := base64.StdEncoding.DecodeString(got["content_base64"].(string))
	if string(decoded) != string(pdf) {
		t.Errorf("content is not the scanned file")
	}
	ctx := got["context"].(map[string]any)
	if ids := ctx["allowed_company_ids"].([]any); len(ids) != 1 || ids[0] != float64(4) {
		t.Errorf("allowed_company_ids = %v", ids)
	}
}

func TestPostToOdooReportsTheOdooMessage(t *testing.T) {
	file := filepath.Join(t.TempDir(), "scanned.pdf")
	os.WriteFile(file, []byte("%PDF"), 0o600)
	withOdoo(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"name": "odoo.exceptions.AccessError", "message": "You cannot archive a document for this company."}`))
	}, []OdooCompany{{ID: 1, Name: "Unstatic Labs"}})

	err := PostToOdoo(1, file)
	if err == nil || !strings.Contains(err.Error(), "You cannot archive a document for this company.") ||
		!strings.Contains(err.Error(), "Unstatic Labs") {
		t.Fatalf("error = %v", err)
	}
}

func TestSendRefusesAnUnconfiguredCompany(t *testing.T) {
	withOdoo(t, func(w http.ResponseWriter, r *http.Request) {}, []OdooCompany{{ID: 3, Name: "Perso"}})
	if err := checkOdooCompanies([]int{3}); err != nil {
		t.Errorf("configured company refused: %v", err)
	}
	if err := checkOdooCompanies([]int{1}); err == nil {
		t.Errorf("unconfigured company accepted")
	}
}

func TestBlankKeyKeepsTheStoredKey(t *testing.T) {
	globalConfig = &Config{DataDir: t.TempDir()}
	settingsMux.Lock()
	appSettings = defaultSettings()
	appSettings.Odoo = OdooConfig{URL: "odoo.example.com", APIKey: "stored"}
	settingsMux.Unlock()

	if err := UpdateSettings(nil, nil, nil, &OdooConfig{URL: "odoo.example.com", Confidentiality: "internal"}); err != nil {
		t.Fatal(err)
	}
	if CurrentSettings().Odoo.APIKey != "stored" {
		t.Errorf("a blank key cleared the stored key")
	}
	if CurrentSettingsRedacted().Odoo.APIKey != "" {
		t.Errorf("the redacted settings expose the key")
	}
}
