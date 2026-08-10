// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var clientset *kubernetes.Clientset

func (s *Server) browserHeadless() string {
	if s != nil && s.config != nil && s.config.BrowserHeadless != "" {
		return s.config.BrowserHeadless
	}
	return "false"
}

// getK8sNamespace returns the configured Kubernetes namespace.
func (s *Server) getK8sNamespace() string {
	if s != nil && s.config != nil && s.config.K8sNamespace != "" {
		return s.config.K8sNamespace
	}
	return "default"
}

// getWorkspacePodImage returns the Docker image to use for workspace pods.
func (s *Server) getWorkspacePodImage() string {
	if s != nil && s.config != nil && s.config.WorkspacePodImage != "" {
		return s.config.WorkspacePodImage
	}
	return "shadowops-workspace-agent:latest"
}

// getWorkspacePodPort returns the agent port configured for workspace pods.
func (s *Server) getWorkspacePodPort() int32 {
	if s != nil && s.config != nil && s.config.WorkspaceAgentPort > 0 {
		return int32(s.config.WorkspaceAgentPort)
	}
	return 8765
}

// getPodResourceRequirements returns ResourceRequirements built from Config.
func (s *Server) getPodResourceRequirements() corev1.ResourceRequirements {
	cpuStr := "200m"
	memStr := "512Mi"
	if s != nil && s.config != nil {
		if s.config.PodCPURequest != "" {
			cpuStr = s.config.PodCPURequest
		}
		if s.config.PodMemoryRequest != "" {
			memStr = s.config.PodMemoryRequest
		}
	}

	cpuReq, err := resource.ParseQuantity(cpuStr)
	if err != nil {
		log.Printf("warning: invalid POD_CPU_REQUEST %q, falling back to 200m: %v", cpuStr, err)
		cpuReq = resource.MustParse("200m")
	}

	memReq, err := resource.ParseQuantity(memStr)
	if err != nil {
		log.Printf("warning: invalid POD_MEMORY_REQUEST %q, falling back to 512Mi: %v", memStr, err)
		memReq = resource.MustParse("512Mi")
	}

	reqs := corev1.ResourceList{
		corev1.ResourceCPU:    cpuReq,
		corev1.ResourceMemory: memReq,
	}

	rr := corev1.ResourceRequirements{
		Requests: reqs,
	}

	if s != nil && s.config != nil {
		limits := corev1.ResourceList{}
		if s.config.PodCPULimit != "" {
			if lim, err := resource.ParseQuantity(s.config.PodCPULimit); err == nil {
				limits[corev1.ResourceCPU] = lim
			} else {
				log.Printf("warning: invalid POD_CPU_LIMIT %q: %v", s.config.PodCPULimit, err)
			}
		}
		if s.config.PodMemoryLimit != "" {
			if lim, err := resource.ParseQuantity(s.config.PodMemoryLimit); err == nil {
				limits[corev1.ResourceMemory] = lim
			} else {
				log.Printf("warning: invalid POD_MEMORY_LIMIT %q: %v", s.config.PodMemoryLimit, err)
			}
		}
		if len(limits) > 0 {
			rr.Limits = limits
		}
	}

	return rr
}

// InitClient initializes the Kubernetes client.
func InitClient() error {
	return InitClientWithConfig(nil)
}

// InitClientWithConfig initializes the Kubernetes client using settings from Config.
func InitClientWithConfig(cfg *Config) error {
	var config *rest.Config
	var err error

	var endpoint, caCertBase64 string
	if cfg != nil {
		endpoint = cfg.GKEEndpoint
		caCertBase64 = cfg.GKECAData
	}
	if endpoint == "" {
		endpoint = os.Getenv("GKE_ENDPOINT")
	}
	if caCertBase64 == "" {
		caCertBase64 = os.Getenv("GKE_CA_CERT")
	}

	if endpoint != "" && caCertBase64 != "" {
		// Running on Cloud Run, construct config manually
		caCert, err := base64.StdEncoding.DecodeString(caCertBase64)
		if err != nil {
			return fmt.Errorf("failed to decode CA cert: %w", err)
		}

		token, err := getMetadataToken()
		if err != nil {
			return fmt.Errorf("failed to get metadata token: %w", err)
		}

		config = &rest.Config{
			Host:        endpoint,
			BearerToken: token,
			TLSClientConfig: rest.TLSClientConfig{
				CAData: caCert,
			},
		}
	} else {
		// Try in-cluster config first
		config, err = rest.InClusterConfig()
		if err != nil {
			// Fallback to kubeconfig for local development
			kubeconfig := os.Getenv("KUBECONFIG")
			if kubeconfig == "" {
				home := os.Getenv("HOME")
				if home != "" {
					kubeconfig = filepath.Join(home, ".kube", "config")
				}
			}
			if kubeconfig != "" {
				config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
				if err != nil {
					return fmt.Errorf("failed to load kubeconfig: %w", err)
				}
			} else {
				return fmt.Errorf("failed to get in-cluster config and no kubeconfig found: %w", err)
			}
		}
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}
	return nil
}

// CreateWorkspacePod creates a new pod in GKE for the workspace.
// If workspaceId is empty, it creates a "free" pod with a random name.
func (s *Server) CreateWorkspacePod(ctx context.Context, workspaceId string, image string) (string, error) {
	if clientset == nil {
		return "", fmt.Errorf("kubernetes client not initialized")
	}

	if image == "" {
		image = s.getWorkspacePodImage()
	}

	port := s.getWorkspacePodPort()
	podName := "workspace-" + workspaceId
	labels := map[string]string{
		"app": "shadowops",
	}
	if workspaceId != "" {
		labels["workspace-id"] = workspaceId
	} else {
		podName = "workspace-free-" + fmt.Sprintf("%d", time.Now().UnixNano())
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   podName,
			Labels: labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:      "agent",
					Image:     image,
					Ports:     []corev1.ContainerPort{{ContainerPort: port}},
					Resources: s.getPodResourceRequirements(),
					Env: []corev1.EnvVar{
						{Name: "AGENT_PORT", Value: fmt.Sprintf("%d", port)},
						{Name: "BROWSER_HEADLESS", Value: s.browserHeadless()},
						{Name: "WORKSPACE_ID", Value: workspaceId},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	result, err := clientset.CoreV1().Pods(s.getK8sNamespace()).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create pod: %w", err)
	}
	return result.Name, nil
}

// ListWorkspacePods lists all pods labeled app=shadowops.
func (s *Server) ListWorkspacePods(ctx context.Context) ([]corev1.Pod, error) {
	if clientset == nil {
		return nil, fmt.Errorf("kubernetes client not initialized")
	}

	listOptions := metav1.ListOptions{
		LabelSelector: "app=shadowops",
	}
	podList, err := clientset.CoreV1().Pods(s.getK8sNamespace()).List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	return podList.Items, nil
}

// AssociatePodWithWorkspace adds the workspace-id label to a pod.
func (s *Server) AssociatePodWithWorkspace(ctx context.Context, podName string, workspaceId string) error {
	if clientset == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	ns := s.getK8sNamespace()
	pod, err := clientset.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels["workspace-id"] = workspaceId

	_, err = clientset.CoreV1().Pods(ns).Update(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update pod labels: %w", err)
	}
	return nil
}

// UnassociatePodFromWorkspace removes the workspace-id label from a pod.
func (s *Server) UnassociatePodFromWorkspace(ctx context.Context, podName string) error {
	if clientset == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	ns := s.getK8sNamespace()
	pod, err := clientset.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	if pod.Labels != nil {
		delete(pod.Labels, "workspace-id")
	}

	_, err = clientset.CoreV1().Pods(ns).Update(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to remove pod labels: %w", err)
	}
	return nil
}

// CreateWorkspaceService creates a ClusterIP service for the workspace pod.
func (s *Server) CreateWorkspaceService(ctx context.Context, workspaceId string) (string, error) {
	if clientset == nil {
		return "", fmt.Errorf("kubernetes client not initialized")
	}

	port := s.getWorkspacePodPort()
	serviceName := "service-" + workspaceId
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
			Labels: map[string]string{
				"app":          "shadowops",
				"workspace-id": workspaceId,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"workspace-id": workspaceId,
			},
			Ports: []corev1.ServicePort{
				{
					Port:       port,
					TargetPort: intstr.FromInt(int(port)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	result, err := clientset.CoreV1().Services(s.getK8sNamespace()).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create service: %w", err)
	}

	return result.Spec.ClusterIP, nil
}

// GetNodeExternalIP returns the external IP of one of the nodes.
func (s *Server) GetNodeExternalIP(ctx context.Context) (string, error) {
	if clientset == nil {
		return "", fmt.Errorf("kubernetes client not initialized")
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list nodes: %w", err)
	}

	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeExternalIP {
				return addr.Address, nil
			}
		}
	}

	return "", fmt.Errorf("no node with external IP found")
}

// DeleteWorkspacePod deletes the pod associated with the workspace.
func (s *Server) DeleteWorkspacePod(ctx context.Context, workspaceId string) error {
	if clientset == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	ns := s.getK8sNamespace()
	// Delete the service first
	serviceName := "service-" + workspaceId
	err := clientset.CoreV1().Services(ns).Delete(ctx, serviceName, metav1.DeleteOptions{})
	if err != nil {
		log.Printf("Failed to delete service %s: %v", serviceName, err)
	}

	// Find the pod by label workspace-id
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("workspace-id=%s", workspaceId),
	}
	pods, err := clientset.CoreV1().Pods(ns).List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("failed to list pods for deletion: %w", err)
	}

	for _, pod := range pods.Items {
		err := clientset.CoreV1().Pods(ns).Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Printf("Failed to delete pod %s: %v", pod.Name, err)
		} else {
			log.Printf("Deleted pod %s associated with workspace %s", pod.Name, workspaceId)
		}
	}

	return nil
}

// CleanupAllWorkspacePods deletes all pods with label app=shadowops.
func (s *Server) CleanupAllWorkspacePods(ctx context.Context) error {
	if clientset == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	ns := s.getK8sNamespace()
	listOptions := metav1.ListOptions{
		LabelSelector: "app=shadowops",
	}
	pods, err := clientset.CoreV1().Pods(ns).List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	for _, pod := range pods.Items {
		err := clientset.CoreV1().Pods(ns).Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Printf("Failed to delete pod %s: %v", pod.Name, err)
		} else {
			log.Printf("Deleted old workspace pod %s", pod.Name)
		}
	}
	return nil
}

// WaitForPodIP waits for the pod to be assigned an IP address.
func (s *Server) WaitForPodIP(ctx context.Context, podName string, timeout time.Duration) (string, error) {
	if clientset == nil {
		return "", fmt.Errorf("kubernetes client not initialized")
	}

	ns := s.getK8sNamespace()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pod, err := clientset.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to get pod: %w", err)
		}
		if pod.Status.PodIP != "" {
			return pod.Status.PodIP, nil
		}
		time.Sleep(1 * time.Second)
	}
	return "", fmt.Errorf("pod IP not assigned within %s", timeout)
}

// getMetadataToken fetches the OAuth token from the metadata server.
func getMetadataToken() (string, error) {
	req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("metadata server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", err
	}

	return tokenResp.AccessToken, nil
}
