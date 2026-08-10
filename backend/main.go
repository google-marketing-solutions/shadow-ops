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

	cfg := LoadConfig(repoRoot)

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatal(err)
	}

	db := InitDB(cfg.DataDir)
	defer db.Close()

	if err := InitClientWithConfig(cfg); err != nil {
		log.Printf("info: Kubernetes client not initialized (standalone mode): %v", err)
	}

	s := NewServerWithConfig(cfg, db)
	if err := s.loadActionSets(); err != nil {
		log.Printf("warning: failed loading action sets: %v", err)
	}
	s.recoverWorkspaces()
	s.cronScheduler.Start()
	s.routes()

	log.Printf("ShadowOps Phase 1 backend listening on %s (data: %s, host-agent: %s, static: %s)",
		cfg.BindAddr, cfg.DataDir, cfg.HostAgentURL, cfg.StaticDir)
	if err := http.ListenAndServe(cfg.BindAddr, s.withCORS(s.withLogging(s.mux))); err != nil {
		log.Fatal(err)
	}
}
