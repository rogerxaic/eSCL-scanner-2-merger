package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/robfig/cron/v3"
	"golang.org/x/exp/slices"
)

var (
	globalConfig *Config
	pdfManager   *PDFManager

	// stateMux guards currentScanRequest, scanResults and scannerStatus,
	// which are mutated from HTTP handlers, the cron job and read by
	// the broadcast goroutines.
	stateMux           sync.Mutex
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
	var err error
	globalConfig, err = SetupScanner()
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// Initialize the PDFManager
	pdfManager = NewPDFManager(globalConfig.DataDir)

	LoadSettings()
	SetupEmail()
	SetupPaperless()
	LoadScanResults()

	// Set up cron job
	c := cron.New()
	_, err = c.AddFunc("0 2 * * *", dailyTask) // Run every day at 2:00 AM
	if err != nil {
		log.Fatalf("error setting up cron job: %v", err)
	}
	c.Start()

	// Safety-net broadcast so clients converge even if an update is missed.
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			broadcastScanResults()
			broadcastScannerStatus()
		}
	}()

	r := gin.Default()
	setupRoutes(r, globalConfig, pdfManager)
	if err := r.Run(); err != nil {
		log.Fatalf("server error: %v", err)
	}
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
		api.POST("/send", handleSend(cfg))
		api.POST("/delete", handleDelete(cfg))
		api.GET("/scanned/:session/:file", handleScannedFile(cfg))
		api.GET("/thumbnails/:session", handleThumbnail(cfg))
		api.GET("/cleanup", handleCleanup)
		api.PATCH("/scan-request", handlePatchScanRequest)
		api.GET("/scan-request", handleGetScanRequest)
		api.GET("/settings", handleGetSettings)
		api.PATCH("/settings", handlePatchSettings)
		api.GET("/odoo/companies", handleOdooCompanies)
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

// isSafeSession rejects names that could escape the data directory.
func isSafeSession(session string) bool {
	return session != "" &&
		!strings.ContainsAny(session, "/\\") &&
		!strings.Contains(session, "..")
}

func snapshotScanResults() []ScanResult {
	stateMux.Lock()
	defer stateMux.Unlock()
	return append([]ScanResult{}, scanResults...)
}

func addScanResults(results ...ScanResult) {
	stateMux.Lock()
	scanResults = append(scanResults, results...)
	stateMux.Unlock()
	saveScanResults()
	broadcastScanResults()
}

func removeScanResults(sessions []string) {
	stateMux.Lock()
	scanResults = slices.DeleteFunc(scanResults, func(r ScanResult) bool {
		return slices.Contains(sessions, r.Session)
	})
	stateMux.Unlock()
	saveScanResults()
	broadcastScanResults()
}

func setScanning(scanning bool) {
	stateMux.Lock()
	scannerStatus.Scanning = scanning
	stateMux.Unlock()
	broadcastScannerStatus()
}

func newScanResult(session string) ScanResult {
	return ScanResult{
		Session:   session,
		Error:     "",
		Thumbnail: fmt.Sprintf("/api/thumbnails/%s", session),
		PDF:       fmt.Sprintf("/api/scanned/%s/scanned.pdf", session),
		CreatedAt: time.Now().Format(time.RFC3339),
		Pages:     countPages(filepath.Join(globalConfig.DataDir, session, "scanned.pdf")),
	}
}

func countPages(pdfPath string) int {
	out, err := exec.Command("pdftk", pdfPath, "dump_data").Output()
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "NumberOfPages:") {
			if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "NumberOfPages:"))); err == nil {
				return n
			}
		}
	}
	return 0
}

func handleSend(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Recipients         []string `json:"recipients"`
			PaperlessInstances []string `json:"paperlessInstances"`
			OdooCompanies      []int    `json:"odooCompanies"`
			Sessions           []string `json:"sessions"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		log.Printf("Send request received: %v", body)
		if len(body.Sessions) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No sessions provided"})
			return
		}
		if len(body.Recipients) == 0 && len(body.PaperlessInstances) == 0 && len(body.OdooCompanies) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No recipients, paperless instances or Odoo companies provided"})
			return
		}
		for _, session := range body.Sessions {
			if !isSafeSession(session) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session"})
				return
			}
		}
		for _, instance := range body.PaperlessInstances {
			if !IsConfiguredPaperlessInstance(instance) {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unknown paperless instance: %s", instance)})
				return
			}
		}
		if err := checkOdooCompanies(body.OdooCompanies); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		failed := map[string][]string{}
		reasons := []string{}
		for _, session := range body.Sessions {
			file := filepath.Join(cfg.DataDir, session, "scanned.pdf")
			if len(body.Recipients) > 0 {
				if !SendEmail(body.Recipients, file) {
					failed[session] = append(failed[session], "email")
					reasons = append(reasons, "email: send failed")
				}
			}
			for _, instance := range body.PaperlessInstances {
				if err := PostToPaperless(instance, file); err != nil {
					log.Printf("paperless upload of %s to %s failed: %v", session, instance, err)
					failed[session] = append(failed[session], instance)
					reasons = append(reasons, err.Error())
				}
			}
			for _, company := range body.OdooCompanies {
				if err := PostToOdoo(company, file); err != nil {
					log.Printf("odoo upload of %s to company %d failed: %v", session, company, err)
					failed[session] = append(failed[session], fmt.Sprintf("odoo:%d", company))
					reasons = append(reasons, err.Error())
				}
			}
		}

		// Only drop the sessions whose sends all succeeded.
		succeeded := []string{}
		for _, session := range body.Sessions {
			if len(failed[session]) == 0 {
				succeeded = append(succeeded, session)
			}
		}
		if len(succeeded) > 0 {
			removeScanResults(succeeded)
		}

		if len(failed) > 0 {
			c.JSON(http.StatusBadGateway, gin.H{
				"error": fmt.Sprintf("%d of %d documents failed to send — %s",
					len(failed), len(body.Sessions), strings.Join(slices.Compact(reasons), "; ")),
				"failedSessions": failed,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Documents sent"})
	}
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
		if len(body.Sessions) != 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Exactly two sessions are required"})
			return
		}
		for _, session := range body.Sessions {
			if !isSafeSession(session) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session"})
				return
			}
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

		// Remove the source thumbnails; the PDFs were already consumed by Merge.
		for _, merged := range body.Sessions {
			os.Remove(filepath.Join(cfg.DataDir, "thumbnails", merged+".jpg"))
		}

		removeScanResults(body.Sessions)
		addScanResults(newScanResult(session))
		c.JSON(http.StatusOK, gin.H{"message": "PDFs merged successfully", "session": session})
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
		if !isSafeSession(body.Session) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session"})
			return
		}
		pdf := filepath.Join(cfg.DataDir, body.Session, "scanned.pdf")
		log.Printf("Splitting PDF: %s", pdf)
		sessions, err := splitPDF(cfg.DataDir, pdf)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		log.Printf("Split PDF into %d files", len(sessions))

		results := []ScanResult{}
		for _, session := range sessions {
			if err := generateThumbnail(cfg.DataDir, session, filepath.Join(cfg.DataDir, session, "scanned.pdf")); err != nil {
				log.Printf("Error generating thumbnail for %s: %v", session, err)
			}
			results = append(results, newScanResult(session))
		}
		addScanResults(results...)
		c.JSON(http.StatusOK, gin.H{"sessions": sessions})
	}
}

// splitPDF bursts a PDF into pages and turns every page into its own regular
// session directory (with a scanned.pdf), so split pages can be merged, sent
// and deleted like any other scan.
func splitPDF(dataDir string, pdf string) ([]string, error) {
	absPDF, err := filepath.Abs(pdf)
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp(dataDir, "split-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	params := []string{
		fmt.Sprintf("A=%s", absPDF),
		"burst",
		"output",
		filepath.Join(tmpDir, "page-%04d.pdf"),
	}
	cmd := exec.Command("pdftk", params...)
	cmd.Dir = tmpDir // pdftk writes doc_data.txt to its working directory
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdftk burst failed: %v", err)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("error reading split directory: %v", err)
	}
	pages := []string{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "page-") && strings.HasSuffix(entry.Name(), ".pdf") {
			pages = append(pages, entry.Name())
		}
	}
	sort.Strings(pages)
	if len(pages) == 0 {
		return nil, fmt.Errorf("split produced no pages")
	}

	base := uuid.NewString()
	sessions := []string{}
	for i, page := range pages {
		session := fmt.Sprintf("%s-%d", base, i+1)
		sessionDir := filepath.Join(dataDir, session)
		if err := os.MkdirAll(sessionDir, 0755); err != nil {
			return nil, err
		}
		if err := os.Rename(filepath.Join(tmpDir, page), filepath.Join(sessionDir, "scanned.pdf")); err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func handleDelete(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Sessions []string `json:"sessions"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(body.Sessions) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No sessions provided"})
			return
		}
		for _, session := range body.Sessions {
			if !isSafeSession(session) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session"})
				return
			}
		}
		for _, session := range body.Sessions {
			if err := os.RemoveAll(filepath.Join(cfg.DataDir, session)); err != nil {
				log.Printf("Error deleting session %s: %v", session, err)
			}
			os.Remove(filepath.Join(cfg.DataDir, "thumbnails", session+".jpg"))
		}
		removeScanResults(body.Sessions)
		c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
	}
}

func handleThumbnail(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := c.Param("session")
		if !isSafeSession(session) {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		filename := filepath.Join(cfg.DataDir, "thumbnails", session+".jpg")
		c.File(filename)
	}
}

func handleCleanup(c *gin.Context) {
	removeExpiredSessions()
	c.JSON(http.StatusOK, gin.H{"message": "Cleanup complete"})
}

func handleGetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, CurrentSettingsRedacted())
}

func handlePatchSettings(c *gin.Context) {
	var updates struct {
		RetentionDays      *int                 `json:"retentionDays"`
		EmailRecipients    *[]string            `json:"emailRecipients"`
		PaperlessInstances *[]PaperlessInstance `json:"paperlessInstances"`
		Odoo               *OdooConfig          `json:"odoo"`
	}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if updates.RetentionDays != nil && (*updates.RetentionDays < 1 || *updates.RetentionDays > 365) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "retentionDays must be between 1 and 365"})
		return
	}
	if updates.Odoo != nil {
		switch updates.Odoo.Confidentiality {
		case "", "internal", "accounting", "private":
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "odoo confidentiality must be internal, accounting or private"})
			return
		}
	}
	if err := UpdateSettings(updates.RetentionDays, updates.EmailRecipients, updates.PaperlessInstances, updates.Odoo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, CurrentSettingsRedacted())
}

// handleOdooCompanies lists the companies the stored API key may use.
func handleOdooCompanies(c *gin.Context) {
	companies, err := FetchOdooCompanies()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, companies)
}

func checkOdooCompanies(companies []int) error {
	for _, company := range companies {
		if !IsConfiguredOdooCompany(company) {
			return fmt.Errorf("unknown Odoo company: %d", company)
		}
	}
	return nil
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

		for _, instance := range req.PaperlessInstances {
			if !IsConfiguredPaperlessInstance(instance) {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unknown paperless instance: %s", instance)})
				return
			}
		}

		if err := checkOdooCompanies(req.OdooCompanies); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		settings := GetSettings(inputSource)
		session := req.Session
		if session == "" {
			session = uuid.NewString()
		} else if !isSafeSession(session) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session"})
			return
		}

		setScanning(true)
		err = Scan(cfg, settings, session, "scanned")
		setScanning(false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if err := generateThumbnail(cfg.DataDir, session, filepath.Join(cfg.DataDir, session, "scanned.pdf")); err != nil {
			log.Printf("%v", err)
		}

		addScanResults(newScanResult(session))

		if req.Duplex && req.Session == "" {
			c.JSON(http.StatusOK, gin.H{"session": session})
		} else {
			file := filepath.Join(cfg.DataDir, session, "scanned.pdf")
			var emailStatus bool
			var paperlessStatus string = "not requested"

			if len(req.Recipients) != 0 {
				emailStatus = SendEmail(req.Recipients, file)
			}
			for _, instance := range req.PaperlessInstances {
				err := PostToPaperless(instance, file)
				if err != nil {
					paperlessStatus = "error"
				} else {
					paperlessStatus = "success"
				}
			}
			odooStatus := "not requested"
			for _, company := range req.OdooCompanies {
				if err := PostToOdoo(company, file); err != nil {
					log.Printf("odoo upload of %s to company %d failed: %v", session, company, err)
					odooStatus = "error"
				} else if odooStatus != "error" {
					odooStatus = "success"
				}
			}
			scheme := "http"
			if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
				scheme = "https"
			}
			c.JSON(http.StatusOK, gin.H{
				"link":            fmt.Sprintf("%s://%s/api/scanned/%s/scanned.pdf", scheme, c.Request.Host, session),
				"emailStatus":     emailStatus,
				"paperlessStatus": paperlessStatus,
				"odooStatus":      odooStatus,
			})
		}
	}
}

func handleScannedFile(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := c.Param("session")
		file := c.Param("file")
		if !isSafeSession(session) || !isSafeSession(file) {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		pdfPath := filepath.Join(cfg.DataDir, session, file)
		log.Printf("Serving file: %s", pdfPath)
		if c.Query("inline") != "" {
			c.File(pdfPath)
			return
		}
		c.FileAttachment(pdfPath, "scanned.pdf")
	}
}

func dailyTask() {
	log.Println("Running daily task...")
	removeExpiredSessions()
	broadcastScannerStatus()
}

func removeExpiredSessions() {
	cutoff := time.Now().AddDate(0, 0, -CurrentSettings().RetentionDays)
	files, err := os.ReadDir(globalConfig.DataDir)
	if err != nil {
		log.Printf("Error reading data dir: %v", err)
		return
	}
	removed := []string{}
	for _, file := range files {
		if !file.IsDir() || file.Name() == "thumbnails" {
			continue
		}
		info, err := file.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		sessionDir := filepath.Join(globalConfig.DataDir, file.Name())
		if err := os.RemoveAll(sessionDir); err != nil {
			log.Printf("Error removing session dir %s: %v", sessionDir, err)
			continue
		}
		os.Remove(filepath.Join(globalConfig.DataDir, "thumbnails", file.Name()+".jpg"))
		removed = append(removed, file.Name())
	}
	if len(removed) > 0 {
		log.Printf("Removed %d expired sessions", len(removed))
		removeScanResults(removed)
	}
}

func handlePatchScanRequest(c *gin.Context) {
	var updates ScanRequest
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stateMux.Lock()
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
	currentScanRequest.OdooCompanies = updates.OdooCompanies
	response := currentScanRequest
	stateMux.Unlock()

	broadcastScanRequest()

	c.JSON(http.StatusOK, response)
}

func handleGetScanRequest(c *gin.Context) {
	stateMux.Lock()
	response := currentScanRequest
	stateMux.Unlock()
	c.JSON(http.StatusOK, response)
}

func handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	// Send initial state before registering the client, so these writes
	// cannot race with broadcast writes.
	stateMux.Lock()
	req := currentScanRequest
	results := append([]ScanResult{}, scanResults...)
	status := scannerStatus
	stateMux.Unlock()
	if err := conn.WriteJSON(BroadcastData{Type: "scanRequest", ScanRequest: &req}); err != nil {
		log.Printf("Failed to send initial state: %v", err)
		return
	}
	conn.WriteJSON(BroadcastData{Type: "scanResult", ScanResults: results})
	conn.WriteJSON(BroadcastData{Type: "scannerStatus", ScannerStatus: &status})

	clientsMux.Lock()
	clients[conn] = true
	clientsMux.Unlock()

	// Remove client when connection closes
	defer func() {
		clientsMux.Lock()
		delete(clients, conn)
		clientsMux.Unlock()
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

func broadcast(data BroadcastData) {
	clientsMux.Lock()
	defer clientsMux.Unlock()

	for client := range clients {
		if err := client.WriteJSON(data); err != nil {
			log.Printf("Failed to send to client: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func broadcastScanRequest() {
	stateMux.Lock()
	req := currentScanRequest
	stateMux.Unlock()
	broadcast(BroadcastData{Type: "scanRequest", ScanRequest: &req})
}

func broadcastScanResults() {
	broadcast(BroadcastData{Type: "scanResult", ScanResults: snapshotScanResults()})
}

func broadcastScannerStatus() {
	stateMux.Lock()
	status := scannerStatus
	stateMux.Unlock()
	broadcast(BroadcastData{Type: "scannerStatus", ScannerStatus: &status})
}

func generateThumbnail(dataDir string, session string, pdfPath string) error {
	thumbnailDir := filepath.Join(dataDir, "thumbnails")

	if err := os.MkdirAll(thumbnailDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %v", err)
	}

	thumbnailPath := filepath.Join(thumbnailDir, session+".jpg")

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
