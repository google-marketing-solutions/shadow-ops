package main

import (
	"database/sql"
	"log"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func InitDB(dataDir string) *sql.DB {
	dbPath := filepath.Join(dataDir, "shadowops.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS action_sets (
		"id" TEXT NOT NULL PRIMARY KEY,
		"name" TEXT,
		"target_app" TEXT,
		"version" INTEGER,
		"status" TEXT,
		"steps" TEXT,
		"created_at" TEXT,
		"updated_at" TEXT,
		"playwright_script" TEXT
	);`

	if _, err := db.Exec(createTableSQL); err != nil {
		log.Fatalf("failed to create action_sets table: %v", err)
	}

	createWorkspaceCronsSQL := `
	CREATE TABLE IF NOT EXISTS workspace_crons (
		"workspace_id" TEXT NOT NULL PRIMARY KEY,
		"cron_expression" TEXT,
		"script_filename" TEXT,
		"script_source" TEXT,
		"csv_filename" TEXT,
		"csv_data" TEXT,
		"created_at" TEXT,
		"updated_at" TEXT
	);`
	if _, err := db.Exec(createWorkspaceCronsSQL); err != nil {
		log.Fatalf("failed to create workspace_crons table: %v", err)
	}

	createBatchExecutionsSQL := `
	CREATE TABLE IF NOT EXISTS batch_executions (
		"id" TEXT NOT NULL PRIMARY KEY,
		"workspace_id" TEXT,
		"started_at" TEXT,
		"finished_at" TEXT,
		"status" TEXT,
		"result_csv_data" TEXT
	);`
	if _, err := db.Exec(createBatchExecutionsSQL); err != nil {
		log.Fatalf("failed to create batch_executions table: %v", err)
	}
	createWorkspacesSQL := `
	CREATE TABLE IF NOT EXISTS workspaces (
		"id" TEXT NOT NULL PRIMARY KEY,
		"name" TEXT,
		"target_app" TEXT,
		"api_key" TEXT,
		"agent_port" INTEGER,
		"container_id" TEXT,
		"agent_base_url" TEXT,
		"created_at" TEXT,
		"updated_at" TEXT
	);`
	if _, err := db.Exec(createWorkspacesSQL); err != nil {
		log.Fatalf("failed to create workspaces table: %v", err)
	}

	// For existing DBs, attempt to add columns safely
	_, _ = db.Exec(`ALTER TABLE workspaces ADD COLUMN api_key TEXT;`)
	_, _ = db.Exec(`ALTER TABLE workspaces ADD COLUMN agent_port INTEGER;`)
	_, _ = db.Exec(`ALTER TABLE workspaces ADD COLUMN container_id TEXT;`)
	_, _ = db.Exec(`ALTER TABLE workspaces ADD COLUMN agent_base_url TEXT;`)

	log.Println("Database initialized successfully.")
	return db
}
