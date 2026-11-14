package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type PaperlessInstance struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
	// Token is a paperless-ngx API token (Profile -> API Auth Token). It takes
	// precedence over Username/Password, and is the only option that works for
	// accounts with MFA enabled — paperless rejects HTTP Basic auth for those.
	Token string `json:"token"`
}

// OdooCompany is one company the send dialog offers as an Odoo destination.
type OdooCompany struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// OdooConfig is the one Odoo instance scans can be archived in. URL is the
// host, like the paperless hosts. Confidentiality is the Odoo level given to
// every upload ("internal" when empty).
type OdooConfig struct {
	URL             string        `json:"url"`
	Database        string        `json:"database"`
	APIKey          string        `json:"apiKey"`
	Confidentiality string        `json:"confidentiality"`
	Companies       []OdooCompany `json:"companies"`
}

type Settings struct {
	RetentionDays      int                 `json:"retentionDays"`
	EmailRecipients    []string            `json:"emailRecipients"`
	PaperlessInstances []PaperlessInstance `json:"paperlessInstances"`
	Odoo               OdooConfig          `json:"odoo"`
}

var (
	appSettings = defaultSettings()
	settingsMux sync.Mutex
)

func defaultSettings() Settings {
	return Settings{
		RetentionDays:      1,
		EmailRecipients:    []string{},
		PaperlessInstances: []PaperlessInstance{},
	}
}

func settingsPath() string {
	return filepath.Join(globalConfig.DataDir, "settings.json")
}

func LoadSettings() {
	data, err := os.ReadFile(settingsPath())
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("error reading settings: %v", err)
		}
		return
	}

	loaded := defaultSettings()
	migrated := false
	if err := json.Unmarshal(data, &loaded); err != nil {
		// Legacy format: paperlessInstances was a plain list of hostnames.
		var legacy struct {
			RetentionDays      int      `json:"retentionDays"`
			PaperlessInstances []string `json:"paperlessInstances"`
		}
		if err2 := json.Unmarshal(data, &legacy); err2 != nil {
			log.Printf("error parsing settings: %v", err)
			return
		}
		loaded = defaultSettings()
		loaded.RetentionDays = legacy.RetentionDays
		loaded.PaperlessInstances = []PaperlessInstance{}
		for _, host := range legacy.PaperlessInstances {
			loaded.PaperlessInstances = append(loaded.PaperlessInstances, PaperlessInstance{Host: host})
		}
		migrated = true
		log.Printf("migrated settings.json from legacy format")
	}

	settingsMux.Lock()
	appSettings = normalizeSettings(loaded)
	settingsMux.Unlock()

	if migrated {
		if err := saveSettings(); err != nil {
			log.Printf("error saving migrated settings: %v", err)
		}
	}
}

func normalizeSettings(s Settings) Settings {
	if s.RetentionDays < 1 {
		s.RetentionDays = 1
	}
	s.EmailRecipients = normalizeEmails(s.EmailRecipients)
	s.PaperlessInstances = normalizeInstances(s.PaperlessInstances)
	s.Odoo = normalizeOdoo(s.Odoo)
	return s
}

func normalizeOdoo(o OdooConfig) OdooConfig {
	o.URL = normalizeHost(o.URL)
	o.Database = strings.TrimSpace(o.Database)
	o.APIKey = strings.TrimSpace(o.APIKey)
	o.Confidentiality = strings.TrimSpace(o.Confidentiality)
	companies := []OdooCompany{}
	for _, company := range o.Companies {
		company.Name = strings.TrimSpace(company.Name)
		if company.ID <= 0 {
			continue
		}
		found := false
		for _, existing := range companies {
			if existing.ID == company.ID {
				found = true
				break
			}
		}
		if !found {
			companies = append(companies, company)
		}
	}
	o.Companies = companies
	return o
}

func normalizeEmails(emails []string) []string {
	out := []string{}
	for _, email := range emails {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}
		found := false
		for _, existing := range out {
			if strings.EqualFold(existing, email) {
				found = true
				break
			}
		}
		if !found {
			out = append(out, email)
		}
	}
	return out
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimSuffix(host, "/")
	return host
}

func normalizeInstances(instances []PaperlessInstance) []PaperlessInstance {
	out := []PaperlessInstance{}
	for _, instance := range instances {
		instance.Host = normalizeHost(instance.Host)
		if instance.Host == "" {
			continue
		}
		instance.Token = strings.TrimSpace(instance.Token)
		found := false
		for _, existing := range out {
			if existing.Host == instance.Host {
				found = true
				break
			}
		}
		if !found {
			out = append(out, instance)
		}
	}
	return out
}

func copySettings(s Settings) Settings {
	s.EmailRecipients = append([]string{}, s.EmailRecipients...)
	s.PaperlessInstances = append([]PaperlessInstance{}, s.PaperlessInstances...)
	s.Odoo.Companies = append([]OdooCompany{}, s.Odoo.Companies...)
	return s
}

func CurrentSettings() Settings {
	settingsMux.Lock()
	defer settingsMux.Unlock()
	return copySettings(appSettings)
}

// RedactedInstance mirrors PaperlessInstance for API responses, with the
// secrets stripped and replaced by "is it set?" flags.
type RedactedInstance struct {
	Host        string `json:"host"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Token       string `json:"token"`
	HasPassword bool   `json:"hasPassword"`
	HasToken    bool   `json:"hasToken"`
}

// RedactedOdoo mirrors OdooConfig for API responses, without the API key.
type RedactedOdoo struct {
	URL             string        `json:"url"`
	Database        string        `json:"database"`
	APIKey          string        `json:"apiKey"`
	HasAPIKey       bool          `json:"hasApiKey"`
	Confidentiality string        `json:"confidentiality"`
	Companies       []OdooCompany `json:"companies"`
}

type RedactedSettings struct {
	RetentionDays      int                `json:"retentionDays"`
	EmailRecipients    []string           `json:"emailRecipients"`
	PaperlessInstances []RedactedInstance `json:"paperlessInstances"`
	Odoo               RedactedOdoo       `json:"odoo"`
}

// CurrentSettingsRedacted is what the API hands out: never the stored password
// or token, only whether one is set. Clients send back an empty string for a
// secret they are not changing; see mergeInstanceSecrets.
func CurrentSettingsRedacted() RedactedSettings {
	current := CurrentSettings()
	out := RedactedSettings{
		RetentionDays:      current.RetentionDays,
		EmailRecipients:    current.EmailRecipients,
		PaperlessInstances: []RedactedInstance{},
		Odoo: RedactedOdoo{
			URL:             current.Odoo.URL,
			Database:        current.Odoo.Database,
			HasAPIKey:       odooAPIKey() != "",
			Confidentiality: current.Odoo.Confidentiality,
			Companies:       current.Odoo.Companies,
		},
	}
	for _, instance := range current.PaperlessInstances {
		out.PaperlessInstances = append(out.PaperlessInstances, RedactedInstance{
			Host:        instance.Host,
			Username:    instance.Username,
			HasPassword: instance.Password != "",
			HasToken:    instance.Token != "",
		})
	}
	return out
}

// mergeInstanceSecrets carries forward the stored password/token for any
// incoming instance that left them blank, so a client working from redacted
// settings can save without wiping credentials it never saw. A secret is
// therefore cleared by removing the instance, not by blanking the field.
// Callers must hold settingsMux.
func mergeInstanceSecrets(incoming []PaperlessInstance) []PaperlessInstance {
	existing := map[string]PaperlessInstance{}
	for _, instance := range appSettings.PaperlessInstances {
		existing[normalizeHost(instance.Host)] = instance
	}
	for i, instance := range incoming {
		prev, ok := existing[normalizeHost(instance.Host)]
		if !ok {
			continue
		}
		if instance.Password == "" {
			incoming[i].Password = prev.Password
		}
		if instance.Token == "" {
			incoming[i].Token = prev.Token
		}
	}
	return incoming
}

func IsConfiguredPaperlessInstance(host string) bool {
	for _, configured := range CurrentSettings().PaperlessInstances {
		if configured.Host == host {
			return true
		}
	}
	return false
}

// saveSettings persists the current settings; callers must NOT hold settingsMux.
func saveSettings() error {
	settingsMux.Lock()
	data, err := json.MarshalIndent(appSettings, "", "  ")
	settingsMux.Unlock()
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath(), data, 0600)
}

func UpdateSettings(retentionDays *int, emailRecipients *[]string, paperlessInstances *[]PaperlessInstance, odoo *OdooConfig) error {
	settingsMux.Lock()
	if retentionDays != nil {
		appSettings.RetentionDays = *retentionDays
	}
	if emailRecipients != nil {
		appSettings.EmailRecipients = *emailRecipients
	}
	if paperlessInstances != nil {
		appSettings.PaperlessInstances = mergeInstanceSecrets(*paperlessInstances)
	}
	if odoo != nil {
		// A blank key keeps the stored one, like the paperless secrets. The
		// key is cleared by removing the URL.
		incoming := *odoo
		if incoming.APIKey == "" && strings.TrimSpace(incoming.URL) != "" {
			incoming.APIKey = appSettings.Odoo.APIKey
		}
		appSettings.Odoo = incoming
	}
	appSettings = normalizeSettings(appSettings)
	settingsMux.Unlock()
	return saveSettings()
}
