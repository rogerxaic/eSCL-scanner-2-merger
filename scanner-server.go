package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/renameio/v2"
	"github.com/google/uuid"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/stapelberg/airscan"
	"github.com/stapelberg/airscan/preset"
)

type InputSource string

const (
	Platen InputSource = "Platen"
	Feeder InputSource = "Feeder"
)

type Config struct {
	Host    string
	DataDir string
	Airscan *airscan.Client
}

func setup() (*Config, error) {
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

func getSettings(inputSource InputSource) *airscan.ScanSettings {
	settings := preset.GrayscaleA4ADF()
	settings.InputSource = string(inputSource)
	settings.ColorMode = "RGB24"
	settings.ScanRegions.Regions[0].Width = 2550
	settings.ScanRegions.Regions[0].Height = 3300
	settings.DocumentFormat = "application/pdf"
	settings.Duplex = false
	return settings
}

func scan(cfg *Config, settings *airscan.ScanSettings, session string, filename string) error {
	scan, err := cfg.Airscan.Scan(settings)
	if err != nil {
		log.Print(err)
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

func main() {
	cfg, err := setup()
	if err != nil {
		log.Fatal("error: %v", err)
		return
	}

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	type ScanRequest struct {
		InputSource string `json:"inputSource"`
		Duplex      bool   `json:"duplex"`
		Session     string `json:"session"`
	}

	r.POST("/scan", func(c *gin.Context) {
		var req ScanRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var scanType string
		if req.Duplex {
			scanType = "recto/verso"
		} else {
			scanType = "only recto"
		}
		log.Printf("scanning %s from %s", scanType, req.InputSource)

		inputSource := InputSource(req.InputSource)
		if inputSource != Platen && inputSource != Feeder {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input source. Please use 'Platen' or 'Feeder'."})
			return
		}

		settings := getSettings(inputSource)

		var session string
		if req.Duplex {
			if req.Session == "" {
				session = uuid.NewString()
				err := scan(cfg, settings, session, "recto")
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"session": session})
				return

			} else {
				session = req.Session
				err := scan(cfg, settings, session, "verso")
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				merge(cfg.DataDir, session)
				file := filepath.Join(cfg.DataDir, session, "scanned.pdf")
				tos := []*mail.Email{mail.NewEmail("Roger", "owner@example.com")}
				emailStatus := SendEmail(tos, file)
				c.JSON(http.StatusOK, gin.H{"link": fmt.Sprintf("http://%s/scanned/%s/scanned.pdf", c.Request.Host, session), "emailStatus": emailStatus})
				return
			}
		} else {
			session = uuid.NewString()
			err := scan(cfg, settings, session, "scanned")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			file := filepath.Join(cfg.DataDir, session, "scanned.pdf")
			tos := []*mail.Email{mail.NewEmail("Roger", "owner@example.com")}
			emailStatus := SendEmail(tos, file)
			// return a link to the scanned file
			c.JSON(http.StatusOK, gin.H{"link": fmt.Sprintf("http://%s/scanned/%s/scanned.pdf", c.Request.Host, session), "emailStatus": emailStatus})
			return
		}

	})

	r.GET("/scanned/:session/:file", func(c *gin.Context) {
		session := c.Param("session")
		sessionDir := filepath.Join(cfg.DataDir, session)
		filename := filepath.Join(sessionDir, c.Param("file"))

		c.FileAttachment(filename, "scanned.pdf")
	})
	r.Run()
	log.Printf("Server started on http://0.0.0.0:8080")
}

func merge(dataDir string, session string) error {
	sessionDir := filepath.Join(dataDir, session)
	// pdftk A=$RECTO B=$VERSO shuffle A Bend-1 output merged_$RECTO
	params := []string{
		fmt.Sprintf("A=%s/recto.pdf", sessionDir),
		fmt.Sprintf("B=%s/verso.pdf", sessionDir),
		"shuffle",
		"A",
		"Bend-1",
		"output",
		fmt.Sprintf("%s/scanned.pdf", sessionDir),
	}
	cmd := exec.Command("pdftk", params...)
	err := cmd.Run()

	if err == nil {
		os.Remove(sessionDir + "/recto.pdf")
		os.Remove(sessionDir + "/verso.pdf")
		return nil
	} else {
		log.Printf("Error merging PDFs: %v", err)
		return err
	}
}
