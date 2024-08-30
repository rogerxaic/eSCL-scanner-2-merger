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
	InputSource string `json:"inputSource"`
	Duplex      bool   `json:"duplex"`
	Session     string `json:"session"`
}
