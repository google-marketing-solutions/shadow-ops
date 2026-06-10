package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"
)

type ActionSet struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	TargetApp        string         `json:"target_app"`
	Version          int            `json:"version"`
	Status           string         `json:"status"`
	Steps            map[string]any `json:"steps"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	PlaywrightScript string         `json:"playwrightScript,omitempty"`
}

func (s *Server) loadActionSets() error {
	rows, err := s.db.Query("SELECT id, name, target_app, version, status, steps, created_at, updated_at, playwright_script FROM action_sets ORDER BY updated_at DESC")
	if err != nil {
		return err
	}
	defer rows.Close()

	s.actionSets = map[string]*ActionSet{}
	for rows.Next() {
		as := &ActionSet{}
		var stepsJSON string
		var createdAt, updatedAt string
		var playwrightScript sql.NullString
		if err := rows.Scan(&as.ID, &as.Name, &as.TargetApp, &as.Version, &as.Status, &stepsJSON, &createdAt, &updatedAt, &playwrightScript); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(stepsJSON), &as.Steps); err != nil {
			log.Printf("failed to unmarshal steps for action set %s: %v", as.ID, err)
			as.Steps = map[string]any{}
		}
		as.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		as.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		if playwrightScript.Valid {
			as.PlaywrightScript = playwrightScript.String
		}
		s.actionSets[as.ID] = as
	}
	return nil
}

func (s *Server) getActionSet(id string) (*ActionSet, bool) {
	s.mu.RLock()
	as, ok := s.actionSets[id]
	s.mu.RUnlock()
	return as, ok
}

func (s *Server) createActionSet(as *ActionSet) error {
	stepsJSON, err := json.Marshal(as.Steps)
	if err != nil {
		return err
	}

	stmt, err := s.db.Prepare("INSERT INTO action_sets(id, name, target_app, version, status, steps, created_at, updated_at, playwright_script) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(as.ID, as.Name, as.TargetApp, as.Version, as.Status, string(stepsJSON), as.CreatedAt.Format(time.RFC3339), as.UpdatedAt.Format(time.RFC3339), as.PlaywrightScript)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.actionSets[as.ID] = as
	s.mu.Unlock()
	return nil
}

func (s *Server) updateActionSet(as *ActionSet) error {
	stepsJSON, err := json.Marshal(as.Steps)
	if err != nil {
		return err
	}

	stmt, err := s.db.Prepare("UPDATE action_sets SET name=?, target_app=?, version=?, status=?, steps=?, updated_at=?, playwright_script=? WHERE id=?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(as.Name, as.TargetApp, as.Version, as.Status, string(stepsJSON), as.UpdatedAt.Format(time.RFC3339), as.PlaywrightScript, as.ID)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.actionSets[as.ID] = as
	s.mu.Unlock()

	return nil
}

func (s *Server) deleteActionSet(id string) error {
	stmt, err := s.db.Prepare("DELETE FROM action_sets WHERE id=?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	if _, err := stmt.Exec(id); err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.actionSets, id)
	s.mu.Unlock()
	return nil
}
