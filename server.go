package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/robfig/cron/v3"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/stapelberg/airscan"
)

var (
	globalConfig       *Config
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

	// Set up cron job
	c := cron.New()
	_, err = c.AddFunc("0 2 * * *", dailyTask) // Run every day at 2:00 AM
	if err != nil {
		log.Fatalf("error setting up cron job: %v", err)
	}
	c.Start()

	r := gin.Default()
	setupRoutes(r, globalConfig)
	r.Run()
	log.Printf("Server started on http://0.0.0.0:8080")
}

func setupRoutes(r *gin.Engine, cfg *Config) {
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
		api.POST("/merge", handleMerge(cfg))
		api.GET("/scanned/:session/:file", handleScannedFile(cfg))
		api.GET("/thumbnails/:session", handleThumbnail(cfg))
		api.GET("/cleanup", handleCleanup)
		api.PATCH("/scan-request", handlePatchScanRequest)
		api.GET("/scan-request", handleGetScanRequest)
		api.GET("/ws", handleWebSocket)
	}

	r.StaticFile("/", "./dist/index.html")
	r.Static("/static", "./dist") // For assets like JS, CSS, images

	// Fallback for SPA routes
	r.NoRoute(func(c *gin.Context) {
		c.File("./dist/index.html")
	})
}

func handleMerge(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var sessions []string
		var body struct {
			Sessions []string `json:"sessions"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		sessions = body.Sessions
		session1 := sessions[0]
		session2 := sessions[1]
		pdf1 := filepath.Join(cfg.DataDir, session1, "scanned.pdf")
		pdf2 := filepath.Join(cfg.DataDir, session2, "scanned.pdf")
		filename, err := merge2PDFs(cfg.DataDir, pdf1, pdf2)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		session := uuid.NewString()

		if err := generateThumbnail(cfg.DataDir, session, filename); err != nil {
			log.Printf("Error generating thumbnail: %v", err)
		}

		// Remove the merged sessions from the results array
		scanResults = slices.DeleteFunc(scanResults, func(r ScanResult) bool {
			return r.Session == session1 || r.Session == session2
		})

		scanResults = append(scanResults, ScanResult{
			Session:   session,
			Error:     "",
			Thumbnail: fmt.Sprintf("http://%s/api/thumbnails/%s", c.Request.Host, session),
			PDF:       fmt.Sprintf("http://%s/api/scanned/%s", c.Request.Host, filename),
		})
		c.JSON(http.StatusOK, gin.H{"message": "PDFs merged successfully"})
	}
}

func handleSplit(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := c.Param("session")
		pdf := filepath.Join(cfg.DataDir, session, "scanned.pdf")
		date, err := splitPDF(cfg.DataDir, pdf)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"date": date})
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
		if err := processScan(cfg, settings, session, req.Duplex); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			scannerStatus.Scanning = false
			broadcastScannerStatus()
			return
		}
		scannerStatus.Scanning = false
		broadcastScannerStatus()

		if err := generateThumbnail(cfg.DataDir, session, fmt.Sprintf("%s/scanned.pdf", session)); err != nil {
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

func processScan(cfg *Config, settings *airscan.ScanSettings, session string, isDuplex bool) error {
	scannerStatus.Scanning = true
	if isDuplex {
		if err := Scan(cfg, settings, session, "recto"); err != nil {
			scannerStatus.Scanning = false
			return err
		}
		if session != "" {
			if err := Scan(cfg, settings, session, "verso"); err != nil {
				scannerStatus.Scanning = false
				return err
			}
			scannerStatus.Scanning = false
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
		log.Printf("Serving file: %s", filename)
		c.FileAttachment(filename, "scanned.pdf")
	}
}

func splitPDF(dataDir string, pdf string) (string, error) {
	date := time.Now().Format("2006-01-02")
	params := []string{
		fmt.Sprintf("A=%s", pdf),
		"burst",
		"output",
		fmt.Sprintf("%s/%s-scan-%%d.pdf", dataDir, date),
	}
	cmd := exec.Command("pdftk", params...)
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return date, nil
}

func merge2PDFs(dataDir string, path1 string, path2 string) (string, error) {
	// pdftk A=$RECTO B=$VERSO shuffle A Bend-1 output merged_$RECTO
	uuid := uuid.NewString()
	date := time.Now().Format("2006-01-02")

	os.MkdirAll(filepath.Join(dataDir, uuid), 0755)

	params := []string{
		fmt.Sprintf("A=%s", path1),
		fmt.Sprintf("B=%s", path2),
		"shuffle",
		"A",
		"Bend-1",
		"output",
		fmt.Sprintf("%s/%s/%s-scan.pdf", dataDir, uuid, date),
	}
	cmd := exec.Command("pdftk", params...)
	err := cmd.Run()

	if err == nil {
		os.Remove(path1)
		os.Remove(path2)
		return fmt.Sprintf("%s/%s-scan.pdf", uuid, date), nil
	} else {
		log.Printf("Error merging PDFs: %v", err)
		return "", err
	}
}

func merge(dataDir string, session string) error {
	sessionDir := filepath.Join(dataDir, session)
	_, err := merge2PDFs(dataDir, sessionDir+"/scanned.pdf", sessionDir+"/scanned.pdf")
	if err != nil {
		return err
	}
	return nil
}

func dailyTask() {
	log.Println("Running daily task...")
	removeAllFilesOlderThanOneDay()
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

func generateThumbnail(dataDir string, session string, filename string) error {
	pdfPath := filepath.Join(dataDir, filename)
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
