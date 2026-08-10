package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *Server) handleListPods(w http.ResponseWriter, r *http.Request) {
	if clientset == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}

	pods, err := s.ListWorkspacePods(r.Context())
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
	if clientset == nil {
		writeError(w, http.StatusServiceUnavailable, "K8S_DISABLED", "Kubernetes client not initialized")
		return
	}

	limit := 10
	if s.config != nil && s.config.PodPoolLimit > 0 {
		limit = s.config.PodPoolLimit
	}
	pods, err := s.ListWorkspacePods(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_PODS_FAILED", err.Error())
		return
	}

	if len(pods) >= limit {
		writeError(w, http.StatusConflict, "POOL_FULL", fmt.Sprintf("Pod pool is full (limit %d). Please delete a pod first.", limit))
		return
	}

	image := s.getWorkspacePodImage()
	podName, err := s.CreateWorkspacePod(r.Context(), "", image)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CREATE_POD_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"name": podName})
}

func (s *Server) handleDeletePod(w http.ResponseWriter, r *http.Request) {
	if clientset == nil {
		writeError(w, http.StatusServiceUnavailable, "K8S_DISABLED", "Kubernetes client not initialized")
		return
	}

	podName := r.PathValue("podName")
	if podName == "" {
		writeError(w, http.StatusBadRequest, "INVALID_POD_NAME", "podName is required")
		return
	}

	ns := s.getK8sNamespace()
	var err error
	if strings.HasPrefix(podName, "workspace-free-") {
		// It's a free pod, no service
		err = clientset.CoreV1().Pods(ns).Delete(r.Context(), podName, metav1.DeleteOptions{})
	} else if strings.HasPrefix(podName, "workspace-") {
		workspaceId := strings.TrimPrefix(podName, "workspace-")
		err = s.DeleteWorkspacePod(r.Context(), workspaceId)
	} else {
		// Unknown pod name format, just try to delete pod
		err = clientset.CoreV1().Pods(ns).Delete(r.Context(), podName, metav1.DeleteOptions{})
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "DELETE_POD_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUpdatePodLabel(w http.ResponseWriter, r *http.Request) {
	if clientset == nil {
		writeError(w, http.StatusServiceUnavailable, "K8S_DISABLED", "Kubernetes client not initialized")
		return
	}

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
	ns := s.getK8sNamespace()
	pod, err := clientset.CoreV1().Pods(ns).Get(r.Context(), podName, metav1.GetOptions{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GET_POD_FAILED", err.Error())
		return
	}

	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels["account"] = req.Account

	_, err = clientset.CoreV1().Pods(ns).Update(r.Context(), pod, metav1.UpdateOptions{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "UPDATE_POD_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
