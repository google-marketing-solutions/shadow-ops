package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	repoRoot, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	// If launched from backend dir, normalize to repo root.
	if filepath.Base(repoRoot) == "backend" {
		repoRoot = filepath.Dir(repoRoot)
	}

	dataDir := filepath.Join(repoRoot, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatal(err)
	}

	db := InitDB(dataDir)
	defer db.Close()

	s := NewServer(repoRoot, dataDir, db)
	if err := s.loadActionSets(); err != nil {
		log.Printf("warning: failed loading action sets: %v", err)
	}
	s.recoverWorkspaces()
	s.cronScheduler.Start()
	s.routes()

	addr := ":8080"
	log.Printf("ShadowOps Phase 1 backend listening on %s", addr)
	if err := http.ListenAndServe(addr, s.withCORS(s.withLogging(s.mux))); err != nil {
		log.Fatal(err)
	}
}
