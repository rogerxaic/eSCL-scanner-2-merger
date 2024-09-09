package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/google/renameio/v2"
	"github.com/stapelberg/airscan"
	"github.com/stapelberg/airscan/preset"
)

const (
	Platen InputSource = "Platen"
	Feeder InputSource = "Feeder"
)

func SetupScanner() (*Config, error) {
	var host string
	var dataDir string

	flag.StringVar(&host, "host", "127.0.0.1", "Host of the printer")
	flag.StringVar(&dataDir, "dataDir", "/scans", "Directory to store scans")
	flag.Parse()

	log.Printf("host: %s", host)
	cl := airscan.NewClient(host)
	return &Config{
		Host:    host,
		DataDir: dataDir,
		Airscan: cl,
	}, nil
}

func GetSettings(inputSource InputSource) *airscan.ScanSettings {
	settings := preset.GrayscaleA4ADF()
	settings.InputSource = string(inputSource)
	settings.ColorMode = "RGB24"
	settings.ScanRegions.Regions[0].Width = 2481
	settings.ScanRegions.Regions[0].Height = 3507
	settings.DocumentFormat = "application/pdf"
	settings.Duplex = false
	return settings
}

func Scan(cfg *Config, settings *airscan.ScanSettings, session string, filename string) error {
	scan, err := cfg.Airscan.Scan(settings)
	if err != nil {
		log.Printf("Airscan.Scan error: %v", err)
		// c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return err
	}
	defer scan.Close()
	sessionDir := filepath.Join(cfg.DataDir, session)
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		if err := os.MkdirAll(sessionDir, 0755); err != nil {
			log.Printf("error creating session directory: %v", err)
			// c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session directory"})
			// return err
			return errors.New("Failed to create session directory")
		}
	}

	for scan.ScanPage() {
		log.Print("receiving page")
		fn := filepath.Join(sessionDir, fmt.Sprintf("%s.%s", filename, "pdf"))

		o, err := renameio.TempFile("", fn)
		if err != nil {
			log.Printf("error: %v", err)
			// c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return errors.New("Failed to create temp file")
		}
		defer o.Cleanup()

		if _, err := io.Copy(o, scan.CurrentPage()); err != nil {
			log.Printf("error: %v", err)
			// c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return errors.New("Failed to copy page to temp file")
		}

		size, err := o.Seek(0, io.SeekCurrent)
		if err != nil {
			log.Printf("error: %v", err)
			// c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return err
		}

		if err := o.CloseAtomicallyReplace(); err != nil {
			log.Printf("error: %v", err)
			// c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return errors.New("Failed to close temp file")
		}

		log.Printf("wrote %s (%d bytes)", fn, size)
	}
	if err := scan.Err(); err != nil {
		log.Printf("error: %v", err)
		// c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return errors.New("Failed to scan page")
	}

	return nil
}
