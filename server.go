package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/stapelberg/airscan"
)

type ScanRequest struct {
	InputSource string `json:"inputSource"`
	Duplex      bool   `json:"duplex"`
	Session     string `json:"session"`
}

func main() {
	SetupEmail()
	cfg, err := SetupScanner()
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	r := gin.Default()
	setupRoutes(r, cfg)
	r.Run()
	log.Printf("Server started on http://0.0.0.0:8080")
}

func setupRoutes(r *gin.Engine, cfg *Config) {
	r.GET("/ping", handlePing)
	r.POST("/scan", handleScan(cfg))
	r.GET("/scanned/:session/:file", handleScannedFile(cfg))
}

func handlePing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "pong"})
}

func handleScan(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ScanRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		inputSource := InputSource(req.InputSource)
		if inputSource != Platen && inputSource != Feeder {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input source. Please use 'Platen' or 'Feeder'."})
			return
		}

		settings := GetSettings(inputSource)
		session := req.Session
		if session == "" {
			session = uuid.NewString()
		}

		if err := processScan(cfg, settings, session, req.Duplex); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if req.Duplex && req.Session == "" {
			c.JSON(http.StatusOK, gin.H{"session": session})
		} else {
			file := filepath.Join(cfg.DataDir, session, "scanned.pdf")
			tos := []*mail.Email{mail.NewEmail("Roger", "owner@example.com")}
			emailStatus := SendEmail(tos, file)
			c.JSON(http.StatusOK, gin.H{
				"link":        fmt.Sprintf("http://%s/scanned/%s/scanned.pdf", c.Request.Host, session),
				"emailStatus": emailStatus,
			})
		}
	}
}

func processScan(cfg *Config, settings *airscan.ScanSettings, session string, isDuplex bool) error {
	if isDuplex {
		if err := Scan(cfg, settings, session, "recto"); err != nil {
			return err
		}
		if session != "" {
			if err := Scan(cfg, settings, session, "verso"); err != nil {
				return err
			}
			return merge(cfg.DataDir, session)
		}
	} else {
		return Scan(cfg, settings, session, "scanned")
	}
	return nil
}

func handleScannedFile(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := c.Param("session")
		filename := filepath.Join(cfg.DataDir, session, c.Param("file"))
		c.FileAttachment(filename, "scanned.pdf")
	}
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
