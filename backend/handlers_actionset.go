package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"
)

func (s *Server) handleListActionSets(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	list := make([]*ActionSet, 0, len(s.actionSets))
	for _, as := range s.actionSets {
		list = append(list, as)
	}
	s.mu.RUnlock()

	sort.Slice(list, func(i, j int) bool {
		return list[j].UpdatedAt.Before(list[i].UpdatedAt)
	})

	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (s *Server) handleGetActionSet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("actionSetId")
	as, ok := s.getActionSet(id)
	if !ok {
		writeError(w, http.StatusNotFound, "ACTION_SET_NOT_FOUND", "action set not found")
		return
	}
	writeJSON(w, http.StatusOK, as)
}

func (s *Server) handleCreateActionSet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string         `json:"name"`
		TargetApp string         `json:"targetApp"`
		Steps     map[string]any `json:"steps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "Untitled Action Set"
	}
	if req.TargetApp == "" {
		req.TargetApp = "google_ads"
	}
	now := time.Now().UTC()
	as := &ActionSet{
		ID:        "as_" + shortID(),
		Name:      req.Name,
		TargetApp: req.TargetApp,
		Version:   1,
		Status:    "active",
		Steps:     req.Steps,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if as.Steps == nil {
		as.Steps = map[string]any{"version": 1, "steps": []any{}}
	}

	if err := s.createActionSet(as); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_INSERT_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"actionSetId": as.ID, "version": as.Version})
}

func (s *Server) handleUpdateActionSet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("actionSetId")
	var req struct {
		Name             *string        `json:"name"`
		Steps            map[string]any `json:"steps"`
		Status           *string        `json:"status"`
		PlaywrightScript *string        `json:"playwrightScript"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON")
		return
	}

	as, ok := s.getActionSet(id)
	if !ok {
		writeError(w, http.StatusNotFound, "ACTION_SET_NOT_FOUND", "action set not found")
		return
	}

	if req.Name != nil {
		as.Name = strings.TrimSpace(*req.Name)
		if as.Name == "" {
			as.Name = "Untitled Action Set"
		}
	}
	if req.Status != nil {
		as.Status = *req.Status
	}
	if req.Steps != nil {
		as.Steps = req.Steps
		as.Version++
	}
	if req.PlaywrightScript != nil {
		as.PlaywrightScript = *req.PlaywrightScript
	}
	as.UpdatedAt = time.Now().UTC()

	if err := s.updateActionSet(as); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"actionSetId": id, "version": as.Version})
}

func (s *Server) handleDeleteActionSet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("actionSetId")
	if _, ok := s.getActionSet(id); !ok {
		writeError(w, http.StatusNotFound, "ACTION_SET_NOT_FOUND", "action set not found")
		return
	}

	if err := s.deleteActionSet(id); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_DELETE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "actionSetId": id})
}

func (s *Server) handleGenerateScript(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("actionSetId")
	as, ok := s.getActionSet(id)
	if !ok {
		writeError(w, http.StatusNotFound, "ACTION_SET_NOT_FOUND", "action set not found")
		return
	}

	var req struct {
		Feedback *GeminiFeedback `json:"feedback"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON")
		return
	}

	script, err := s.generatePlaywrightScript(as, req.Feedback)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SCRIPT_GENERATION_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"script": script})
}
