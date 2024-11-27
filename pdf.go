package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
)

type PDFManager struct {
	dataDir string
}

func NewPDFManager(dataDir string) *PDFManager {
	return &PDFManager{
		dataDir: dataDir,
	}
}

func (pm *PDFManager) Split(pdfPath string) (string, int, error) {
	session := uuid.NewString()
	sessionDir := filepath.Join(pm.dataDir, session)

	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return "", 0, fmt.Errorf("failed to create session directory: %v", err)
	}

	log.Printf("Splitting PDF: %s", pdfPath)
	params := []string{
		fmt.Sprintf("A=%s", pdfPath),
		"burst",
		"output",
		filepath.Join(sessionDir, "scanned-%d.pdf"),
	}

	cmd := exec.Command("pdftk", params...)
	if err := cmd.Run(); err != nil {
		return "", 0, fmt.Errorf("failed to split PDF: %v", err)
	}

	files, err := os.ReadDir(sessionDir)
	if err != nil {
		return "", 0, fmt.Errorf("error reading session directory: %v", err)
	}

	numFiles := len(files)
	log.Printf("Split PDF: %s into %d files", session, numFiles)
	return session, numFiles, nil
}

func (pm *PDFManager) Merge(path1, path2 string) (outputPath string, session string, err error) {
	session = uuid.NewString()
	sessionDir := filepath.Join(pm.dataDir, session)
	outputPath = filepath.Join(sessionDir, "scanned.pdf")

	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create session directory: %v", err)
	}

	params := []string{
		fmt.Sprintf("A=%s", path1),
		fmt.Sprintf("B=%s", path2),
		"shuffle",
		"A",
		"Bend-1",
		"output",
		outputPath,
	}

	cmd := exec.Command("pdftk", params...)
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to merge PDFs: %v", err)
	}

	// Clean up original files
	os.Remove(path1)
	os.Remove(path2)

	return outputPath, session, nil
}

func (pm *PDFManager) GenerateThumbnail(session string, pdfPath string) error {
	thumbnailDir := filepath.Join(pm.dataDir, "thumbnails")

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
