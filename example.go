package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/renameio/v2"
	"github.com/stapelberg/airscan"
	"github.com/stapelberg/airscan/preset"
)

type InputSource string

const (
	Platen InputSource = "Platen"
	Feeder InputSource = "Feeder"
)

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

func main() {

	printer_in_lan := "1.2.3.4"
	printer_via_ssh := "127.0.0.1:1234"
	available_printers := []string{printer_in_lan, printer_via_ssh}

	var host string
	flag.StringVar(&host, "host", available_printers[0], "Host of the printer")
	flag.Parse()

	log.Printf("host: %s", host)
	cl := airscan.NewClient(host)

	// return nil

	// cl := airscan.NewClientForService(scanner)
	// println(cl)

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	r.GET("/scan/:inputSource", func(c *gin.Context) {
		inputSource := InputSource(c.Param("inputSource"))
		if inputSource != Platen && inputSource != Feeder {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input source. Please use 'Platen' or 'Feeder'."})
			return
		}
		settings := getSettings(inputSource)
		scan, err := cl.Scan(settings)
		if err != nil {
			log.Print(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer scan.Close()

		pagenum := 1
		for scan.ScanPage() {
			log.Printf("receiving page %d", pagenum)
			var fn string
			for {
				fn = filepath.Join("/tmp", fmt.Sprintf("page%d.%s", pagenum, "pdf"))
				_, err := os.Stat(fn)
				if err == nil /* file exists */ {
					pagenum++
					continue
				}
				if os.IsNotExist(err) {
					break
				}
			}

			o, err := renameio.TempFile("", fn)
			if err != nil {
				log.Printf("error: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer o.Cleanup()

			if _, err := io.Copy(o, scan.CurrentPage()); err != nil {
				log.Printf("error: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			size, err := o.Seek(0, io.SeekCurrent)
			if err != nil {
				log.Printf("error: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			if err := o.CloseAtomicallyReplace(); err != nil {
				log.Printf("error: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			log.Printf("wrote %s (%d bytes)", fn, size)

			pagenum++
		}
		if err := scan.Err(); err != nil {
			log.Printf("error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "scan complete"})
	})

	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
