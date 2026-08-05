package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// The scan list lives in memory but is mirrored to disk, so a restart (deploy,
// crash, reboot) does not lose it. On startup the stored list is reconciled
// against what is actually in the data dir: entries whose PDF is gone are
// dropped, and sessions on disk that the file does not know about are adopted.
// That second half also covers scans made before this file existed.
//
// Sending a scan removes it from the list but deliberately leaves the PDF on
// disk (retention handles deletion), so "not in the list" alone cannot mean
// "adopt it". Dismissed sessions are tombstoned here, otherwise every restart
// would resurrect everything the user had already sent.

type scanStore struct {
	Results   []ScanResult `json:"results"`
	Dismissed []string     `json:"dismissed"`
}

var dismissedSessions = map[string]bool{}

func scanResultsPath() string {
	return filepath.Join(globalConfig.DataDir, "scanresults.json")
}

// saveScanResults persists the current list. Callers must NOT hold stateMux.
func saveScanResults() {
	results := snapshotScanResults()

	live := map[string]bool{}
	for _, result := range results {
		live[result.Session] = true
	}
	// Read the directory before taking the lock — no file I/O under stateMux,
	// which the broadcast goroutines also contend for.
	onDisk := sessionsOnDisk()

	// A session that is on disk but no longer in the list has been dismissed.
	stateMux.Lock()
	for session := range onDisk {
		if !live[session] {
			dismissedSessions[session] = true
		}
	}
	dismissed := []string{}
	for session := range dismissedSessions {
		dismissed = append(dismissed, session)
	}
	stateMux.Unlock()
	sort.Strings(dismissed)

	data, err := json.MarshalIndent(scanStore{Results: results, Dismissed: dismissed}, "", "  ")
	if err != nil {
		log.Printf("error encoding scan results: %v", err)
		return
	}
	// Write-then-rename so an interrupted write cannot truncate the list.
	tmp := scanResultsPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		log.Printf("error writing scan results: %v", err)
		return
	}
	if err := os.Rename(tmp, scanResultsPath()); err != nil {
		log.Printf("error saving scan results: %v", err)
	}
}

// sessionsOnDisk maps session -> mtime of its scanned.pdf.
func sessionsOnDisk() map[string]time.Time {
	out := map[string]time.Time{}
	entries, err := os.ReadDir(globalConfig.DataDir)
	if err != nil {
		log.Printf("error reading data dir for scan results: %v", err)
		return out
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "thumbnails" {
			continue
		}
		info, err := os.Stat(filepath.Join(globalConfig.DataDir, entry.Name(), "scanned.pdf"))
		if err != nil {
			continue
		}
		out[entry.Name()] = info.ModTime()
	}
	return out
}

// LoadScanResults restores the scan list at startup. It never deletes anything
// from disk — a session it cannot make sense of is simply adopted with what can
// be recovered from the filesystem.
func LoadScanResults() {
	store := scanStore{}
	if data, err := os.ReadFile(scanResultsPath()); err == nil {
		if err := json.Unmarshal(data, &store); err != nil {
			log.Printf("error parsing scan results, rebuilding from disk: %v", err)
			store = scanStore{}
		}
	} else if !os.IsNotExist(err) {
		log.Printf("error reading scan results, rebuilding from disk: %v", err)
	}
	stored := store.Results

	onDisk := sessionsOnDisk()

	stateMux.Lock()
	dismissedSessions = map[string]bool{}
	for _, session := range store.Dismissed {
		// Drop tombstones whose PDF is gone so the file cannot grow forever.
		if _, ok := onDisk[session]; ok {
			dismissedSessions[session] = true
		}
	}
	stateMux.Unlock()

	restored := []ScanResult{}
	seen := map[string]bool{}
	for _, result := range stored {
		if _, ok := onDisk[result.Session]; !ok {
			// PDF is gone (retention cron, manual delete): drop the entry.
			continue
		}
		if seen[result.Session] {
			continue
		}
		seen[result.Session] = true
		restored = append(restored, result)
	}

	adopted := 0
	for session, modTime := range onDisk {
		if seen[session] || dismissedSessions[session] {
			continue
		}
		result := newScanResult(session)
		result.CreatedAt = modTime.Format(time.RFC3339)
		restored = append(restored, result)
		adopted++
	}

	sort.Slice(restored, func(i, j int) bool {
		return scanResultTime(restored[i], onDisk).Before(scanResultTime(restored[j], onDisk))
	})

	for _, result := range restored {
		thumbnail := filepath.Join(globalConfig.DataDir, "thumbnails", result.Session+".jpg")
		if _, err := os.Stat(thumbnail); err == nil {
			continue
		}
		pdf := filepath.Join(globalConfig.DataDir, result.Session, "scanned.pdf")
		if err := generateThumbnail(globalConfig.DataDir, result.Session, pdf); err != nil {
			log.Printf("could not regenerate thumbnail for %s: %v", result.Session, err)
		}
	}

	stateMux.Lock()
	scanResults = restored
	stateMux.Unlock()

	log.Printf("restored %d scan result(s) (%d adopted from disk)", len(restored), adopted)
	// Persist the reconciled list so the adopted sessions survive the next restart.
	saveScanResults()
}

// scanResultTime is the result's CreatedAt, falling back to the PDF mtime when
// it is missing or unparseable, so ordering never silently collapses.
func scanResultTime(result ScanResult, onDisk map[string]time.Time) time.Time {
	if parsed, err := time.Parse(time.RFC3339, result.CreatedAt); err == nil {
		return parsed
	}
	return onDisk[result.Session]
}
