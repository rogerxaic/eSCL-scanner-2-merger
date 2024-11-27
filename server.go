package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/robfig/cron/v3"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/stapelberg/airscan"
	"golang.org/x/exp/slices"
)

var (
	globalConfig       *Config
	pdfManager         *PDFManager
	currentScanRequest = ScanRequest{
		PaperlessInstances: []string{},
	}
	scanResults   = []ScanResult{}
	scannerStatus = ScannerStatus{
		Scanning: false,
	}
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Be careful with this in production
		},
	}
	clients    = make(map[*websocket.Conn]bool)
	clientsMux sync.Mutex
)

func main() {
	SetupEmail()
	SetupPaperless()
	var err error
	globalConfig, err = SetupScanner()
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// Initialize the PDFManager
	pdfManager = NewPDFManager(globalConfig.DataDir)

	// Set up cron job
	c := cron.New()
	_, err = c.AddFunc("0 2 * * *", dailyTask) // Run every day at 2:00 AM
	if err != nil {
		log.Fatalf("error setting up cron job: %v", err)
	}
	c.Start()

	r := gin.Default()
	setupRoutes(r, globalConfig, pdfManager)
	r.Run()
	log.Printf("Server started on http://0.0.0.0:8080")
}

func setupRoutes(r *gin.Engine, cfg *Config, pdfManager *PDFManager) {
	// Add CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	api := r.Group("/api")
	{
		api.GET("/ping", handlePing)
		api.POST("/scan", handleScan(cfg))
		api.POST("/split", handleSplit(cfg))
		api.POST("/merge", handleMerge(cfg, pdfManager))
		api.GET("/scanned/:session/:file", handleScannedFile(cfg))
		api.GET("/thumbnails/:session", handleThumbnail(cfg))
		api.GET("/cleanup", handleCleanup)
		api.PATCH("/scan-request", handlePatchScanRequest)
		api.GET("/scan-request", handleGetScanRequest)
		api.GET("/ws", handleWebSocket)
	}

	r.StaticFile("/", "./dist/index.html")
	r.StaticFile("/favicon.ico", "./dist/favicon.ico")
	r.StaticFile("/manifest.json", "./dist/manifest.json")
	r.StaticFile("/asset-manifest.json", "./dist/asset-manifest.json")
	r.StaticFile("/logo192.png", "./dist/logo192.png")
	r.StaticFile("/logo512.png", "./dist/logo512.png")
	r.StaticFile("/robots.txt", "./dist/robots.txt")
	r.Static("/static", "./dist/static") // For assets like JS, CSS, images

	// Fallback for SPA routes
	r.NoRoute(func(c *gin.Context) {
		c.File("./dist/index.html")
	})
}

func handleMerge(cfg *Config, pdfManager *PDFManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Sessions []string `json:"sessions"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		pdf1 := filepath.Join(cfg.DataDir, body.Sessions[0], "scanned.pdf")
		pdf2 := filepath.Join(cfg.DataDir, body.Sessions[1], "scanned.pdf")

		pdfPath, session, err := pdfManager.Merge(pdf1, pdf2)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if err := pdfManager.GenerateThumbnail(session, pdfPath); err != nil {
			log.Printf("Error generating thumbnail: %v", err)
		}

		// Update scan results...
		updateScanResults(c, session, body.Sessions)
		c.JSON(http.StatusOK, gin.H{"message": "PDFs merged successfully"})
	}
}

func handleSplit(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Session string `json:"session"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		session := body.Session
		pdf := filepath.Join(cfg.DataDir, session, "scanned.pdf")
		log.Printf("Splitting PDF: %s", pdf)
		session, numFiles, err := splitPDF(cfg.DataDir, pdf)
		log.Printf("Split PDF: %s into %d files", session, numFiles)
		appendSession := session
		for i := 1; i <= numFiles; i++ {
			appendSession = fmt.Sprintf("%s-%d", session, i)
			scanResults = append(scanResults, ScanResult{
				Session:   appendSession,
				Error:     "",
				Thumbnail: fmt.Sprintf("http://%s/api/thumbnails/%s", c.Request.Host, appendSession),
				PDF:       fmt.Sprintf("http://%s/api/scanned/%s/scanned.pdf", c.Request.Host, appendSession),
			})
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"session": session})
	}
}

func handleThumbnail(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := c.Param("session")
		filename := filepath.Join(cfg.DataDir, "thumbnails", session+".jpg")
		c.File(filename)
	}
}

func handleCleanup(c *gin.Context) {
	removeAllFilesOlderThanOneDay()
	c.JSON(http.StatusOK, gin.H{"message": "Cleanup complete"})
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

		status, err := cfg.Airscan.ScannerStatus()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		inputSource := InputSource(req.InputSource)
		if inputSource == "Auto" {
			if status.ADFState == "ScannerAdfEmpty" {
				inputSource = Platen
			} else {
				inputSource = Feeder
			}
		}

		if inputSource != Platen && inputSource != Feeder {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input source. Please use 'Platen' or 'Feeder'."})
			return
		}

		settings := GetSettings(inputSource)
		session := req.Session
		if session == "" {
			session = uuid.NewString()
		}
		broadcastScannerStatus()
		if err := processScan(cfg, settings, session); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			scannerStatus.Scanning = false
			broadcastScannerStatus()
			return
		}
		scannerStatus.Scanning = false
		broadcastScannerStatus()

		if err := generateThumbnail(cfg.DataDir, session, filepath.Join(cfg.DataDir, session, "scanned.pdf")); err != nil {
			log.Printf("%v", err)
		}

		scanResults = append(scanResults, ScanResult{
			Session:   session,
			Error:     "",
			Thumbnail: fmt.Sprintf("http://%s/api/thumbnails/%s", c.Request.Host, session),
			PDF:       fmt.Sprintf("http://%s/api/scanned/%s/scanned.pdf", c.Request.Host, session),
		})
		broadcastScanResults()

		if req.Duplex && req.Session == "" {
			c.JSON(http.StatusOK, gin.H{"session": session})
		} else {
			file := filepath.Join(cfg.DataDir, session, "scanned.pdf")
			var emailStatus bool
			var paperlessStatus string = "not requested"

			tos := []*mail.Email{}
			for _, recipient := range req.Recipients {
				tos = append(tos, mail.NewEmail(recipient, recipient))
			}
			if len(tos) != 0 {
				// tos = append(tos, mail.NewEmail("Roger", "owner@example.com"))
				emailStatus = SendEmail(tos, file)
			}
			for _, instance := range req.PaperlessInstances {
				err := PostToPaperless("https://"+instance, file)
				if err != nil {
					paperlessStatus = "error"
				} else {
					paperlessStatus = "success"
				}
			}
			c.JSON(http.StatusOK, gin.H{
				"link":            fmt.Sprintf("http://%s/scanned/%s/scanned.pdf", c.Request.Host, session),
				"emailStatus":     emailStatus,
				"paperlessStatus": paperlessStatus,
			})
		}
	}
}

func processScan(cfg *Config, settings *airscan.ScanSettings, session string) error {
	scannerStatus.Scanning = true
	return Scan(cfg, settings, session, "scanned")
}

func handleScannedFile(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := c.Param("session")
		pdfPath := filepath.Join(cfg.DataDir, session, c.Param("file"))
		log.Printf("Serving file: %s", pdfPath)
		c.FileAttachment(pdfPath, "scanned.pdf")
	}
}

func splitPDF(dataDir string, pdf string) (string, int, error) {
	session := uuid.NewString()

	os.MkdirAll(filepath.Join(dataDir, session), 0755)

	log.Printf("Splitting PDF: %s", pdf)
	params := []string{
		fmt.Sprintf("A=%s", pdf),
		"burst",
		"output",
		fmt.Sprintf("%s/%s/scanned-%%d.pdf", dataDir, session),
	}
	log.Printf("pdftk params: %v", params)
	cmd := exec.Command("pdftk", params...)
	err := cmd.Run()
	if err != nil {
		return "", 0, err
	}
	// Count number of files in session directory
	files, err := os.ReadDir(filepath.Join(dataDir, session))
	if err != nil {
		return "", 0, fmt.Errorf("error reading session directory: %v", err)
	}
	numFiles := len(files)
	log.Printf("Split PDF: %s into %d files.", session, numFiles)
	return session, numFiles, nil
}

func dailyTask() {
	log.Println("Running daily task...")
	removeAllFilesOlderThanOneDay()
	scanResults = []ScanResult{}
	broadcastScanResults()
	broadcastScannerStatus()
}

func removeAllFilesOlderThanOneDay() {
	files, err := os.ReadDir(globalConfig.DataDir)
	if err != nil {
		log.Printf("Error reading data dir: %v", err)
		return
	}
	for _, file := range files {
		if file.IsDir() {
			sessionDir := filepath.Join(globalConfig.DataDir, file.Name())
			os.RemoveAll(sessionDir)
		}
	}
}

func handlePatchScanRequest(c *gin.Context) {
	var updates ScanRequest
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if updates.InputSource != "" {
		currentScanRequest.InputSource = updates.InputSource
	}
	if updates.Session != "" {
		currentScanRequest.Session = updates.Session
	}
	if len(updates.Recipients) > 0 {
		currentScanRequest.Recipients = updates.Recipients
	}
	currentScanRequest.Duplex = updates.Duplex
	currentScanRequest.PaperlessInstances = updates.PaperlessInstances

	// After updating currentScanRequest, broadcast the changes
	go broadcastScanRequest()

	c.JSON(http.StatusOK, currentScanRequest)
}

func handleGetScanRequest(c *gin.Context) {
	c.JSON(http.StatusOK, currentScanRequest)
}

func handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	clientsMux.Lock()
	clients[conn] = true
	clientsMux.Unlock()

	// Remove client when connection closes
	defer func() {
		clientsMux.Lock()
		delete(clients, conn)
		clientsMux.Unlock()
	}()

	// Send initial state
	if err := conn.WriteJSON(currentScanRequest); err != nil {
		log.Printf("Failed to send initial state: %v", err)
		return
	}

	go broadcastScanRequest()
	go broadcastScanResults()
	go broadcastScannerStatus()

	// Start a ticker to periodically broadcast data
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			// broadcastScanRequest()
			broadcastScanResults()
			broadcastScannerStatus()
		}
	}()

	// Keep connection alive
	for {
		// Read messages (required to keep connection alive)
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func broadcastScanRequest() {
	clientsMux.Lock()
	defer clientsMux.Unlock()

	for client := range clients {
		err := client.WriteJSON(BroadcastData{
			Type:        "scanRequest",
			ScanRequest: &currentScanRequest,
		})
		if err != nil {
			log.Printf("Failed to send to client: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func broadcastScanResults() {
	clientsMux.Lock()
	defer clientsMux.Unlock()

	for client := range clients {
		err := client.WriteJSON(BroadcastData{
			Type:        "scanResult",
			ScanResults: scanResults,
		})
		if err != nil {
			log.Printf("Failed to send to client: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func broadcastScannerStatus() {
	clientsMux.Lock()
	defer clientsMux.Unlock()

	for client := range clients {
		err := client.WriteJSON(BroadcastData{
			Type:          "scannerStatus",
			ScannerStatus: &scannerStatus,
		})
		if err != nil {
			log.Printf("Failed to send to client: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func generateThumbnail(dataDir string, session string, pdfPath string) error {
	thumbnailDir := filepath.Join(dataDir, "thumbnails")

	if err := os.MkdirAll(thumbnailDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %v", err)
	}

	thumbnailPath := filepath.Join(thumbnailDir, session+".jpg")

	log.Printf("pdfPath: %s", pdfPath)
	log.Printf("thumbnailPath: %s", thumbnailPath)

	cmd := exec.Command(
		"convert",
		"-density", "150",
		"-quality", "90",
		"-resize", "300x",
		pdfPath+"[0]",
		thumbnailPath,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate thumbnail: %v", err)
	}

	return nil
}

func updateScanResults(c *gin.Context, newSession string, mergedSessions []string) {
	// Remove the merged sessions from the results array
	scanResults = slices.DeleteFunc(scanResults, func(r ScanResult) bool {
		for _, session := range mergedSessions {
			if r.Session == session {
				return true
			}
		}
		return false
	})

	// Add the new merged result
	scanResults = append(scanResults, ScanResult{
		Session:   newSession,
		Error:     "",
		Thumbnail: fmt.Sprintf("http://%s/api/thumbnails/%s", c.Request.Host, newSession),
		PDF:       fmt.Sprintf("http://%s/api/scanned/%s/scanned.pdf", c.Request.Host, newSession),
	})

	// Broadcast the updated results
	broadcastScanResults()
}
