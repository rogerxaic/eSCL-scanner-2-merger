package main

import (
	"github.com/stapelberg/airscan"
)

type Config struct {
	Host    string
	DataDir string
	Airscan *airscan.Client
}

type InputSource string

type ScanRequest struct {
	InputSource        string   `json:"inputSource"`
	Duplex             bool     `json:"duplex"`
	Session            string   `json:"session"`
	Recipients         []string `json:"recipients"`
	PaperlessInstances []string `json:"paperlessInstances"`
	OdooCompanies      []int    `json:"odooCompanies"`
}

type ScanResult struct {
	Session   string `json:"session"`
	Error     string `json:"error"`
	Thumbnail string `json:"thumbnail"`
	PDF       string `json:"pdf"`
	CreatedAt string `json:"createdAt"`
	Pages     int    `json:"pages"`
}

type BroadcastData struct {
	Type          string         `json:"type"` // can be "scanResult" or "scanRequest" or "scannerStatus"
	ScanResults   []ScanResult   `json:"scanResults,omitempty"`
	ScanRequest   *ScanRequest   `json:"scanRequest,omitempty"`
	ScannerStatus *ScannerStatus `json:"scannerStatus,omitempty"`
}

type ScannerStatus struct {
	Scanning bool `json:"scanning"`
}
