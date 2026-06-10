package main

import (
	"encoding/json"
	"net/http"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *Server) handleListPods(w http.ResponseWriter, r *http.Request) {
	pods, err := ListWorkspacePods(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_PODS_FAILED", err.Error())
		return
	}

	list := make([]map[string]any, 0, len(pods))
	for _, pod := range pods {
		workspaceId := ""
		account := ""
		if pod.Labels != nil {
			workspaceId = pod.Labels["workspace-id"]
			account = pod.Labels["account"]
		}
		item := map[string]any{
			"name":        pod.Name,
			"status":      pod.Status.Phase,
			"workspaceId": workspaceId,
			"account":     account,
			"createdAt":   pod.CreationTimestamp.Time,
		}
		list = append(list, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (s *Server) handleCreatePod(w http.ResponseWriter, r *http.Request) {
	// Enforce pool limit of 10 pods
	pods, err := ListWorkspacePods(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_PODS_FAILED", err.Error())
		return
	}

	if len(pods) >= 10 {
		writeError(w, http.StatusConflict, "POOL_FULL", "Pod pool is full (limit 10). Please delete a pod first.")
		return
	}

	image := "us-central1-docker.pkg.dev/shrxx-experiment-sanbox-826381/hello-world-repo/workspace:v1"
	podName, err := CreateWorkspacePod(r.Context(), "", image) // Empty workspaceId creates a free pod
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CREATE_POD_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"name": podName})
}

func (s *Server) handleDeletePod(w http.ResponseWriter, r *http.Request) {
	podName := r.PathValue("podName")
	if podName == "" {
		writeError(w, http.StatusBadRequest, "INVALID_POD_NAME", "podName is required")
		return
	}

	// We need to find the workspace ID associated with this pod if any, to clean up service too!
	// Or we can just delete the pod and let the service be deleted by name if it matches!
	// DeleteWorkspacePod now deletes both pod and service!
	// But it expects workspaceId to construct names!
	// If we only have podName, we might need to extract workspaceId from labels!
	// Let's check if we can delete by pod name directly!
	// In gke.go:
	// podName := "workspace-" + workspaceId
	// serviceName := "service-" + workspaceId
	// So if we have podName, and it starts with "workspace-", we can extract workspaceId!
	// Wait, free pods are named "workspace-free-...". They don't have services!
	// So if it's a free pod, we only delete the pod!

	var err error
	if strings.HasPrefix(podName, "workspace-free-") {
		// It's a free pod, no service
		err = clientset.CoreV1().Pods("default").Delete(r.Context(), podName, metav1.DeleteOptions{})
	} else if strings.HasPrefix(podName, "workspace-") {
		workspaceId := strings.TrimPrefix(podName, "workspace-")
		err = DeleteWorkspacePod(r.Context(), workspaceId)
	} else {
		// Unknown pod name format, just try to delete pod
		err = clientset.CoreV1().Pods("default").Delete(r.Context(), podName, metav1.DeleteOptions{})
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "DELETE_POD_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUpdatePodLabel(w http.ResponseWriter, r *http.Request) {
	podName := r.PathValue("podName")
	if podName == "" {
		writeError(w, http.StatusBadRequest, "INVALID_POD_NAME", "podName is required")
		return
	}

	var req struct {
		Account string `json:"account"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Update pod label in K8s
	pod, err := clientset.CoreV1().Pods("default").Get(r.Context(), podName, metav1.GetOptions{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GET_POD_FAILED", err.Error())
		return
	}

	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels["account"] = req.Account

	_, err = clientset.CoreV1().Pods("default").Update(r.Context(), pod, metav1.UpdateOptions{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "UPDATE_POD_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
