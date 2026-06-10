package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var clientset *kubernetes.Clientset

// InitClient initializes the Kubernetes client.
// It tries GKE_ENDPOINT env var first (for Cloud Run), then in-cluster config, then kubeconfig.
func InitClient() error {
	var config *rest.Config
	var err error

	endpoint := os.Getenv("GKE_ENDPOINT")
	caCertBase64 := os.Getenv("GKE_CA_CERT")

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
func CreateWorkspacePod(ctx context.Context, workspaceId string, image string) (string, error) {
	if clientset == nil {
		return "", fmt.Errorf("kubernetes client not initialized")
	}

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
					Name:  "agent",
					Image: image,
					Ports: []corev1.ContainerPort{{ContainerPort: 8765}},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
					},
					Env: []corev1.EnvVar{
						{Name: "AGENT_PORT", Value: "8765"},
						{Name: "BROWSER_HEADLESS", Value: "false"},
						{Name: "WORKSPACE_ID", Value: workspaceId},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	result, err := clientset.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create pod: %w", err)
	}
	return result.Name, nil
}

// ListWorkspacePods lists all pods labeled app=shadowops.
func ListWorkspacePods(ctx context.Context) ([]corev1.Pod, error) {
	if clientset == nil {
		return nil, fmt.Errorf("kubernetes client not initialized")
	}

	listOptions := metav1.ListOptions{
		LabelSelector: "app=shadowops",
	}
	podList, err := clientset.CoreV1().Pods("default").List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	return podList.Items, nil
}

// AssociatePodWithWorkspace adds the workspace-id label to a pod.
func AssociatePodWithWorkspace(ctx context.Context, podName string, workspaceId string) error {
	if clientset == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	pod, err := clientset.CoreV1().Pods("default").Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels["workspace-id"] = workspaceId

	_, err = clientset.CoreV1().Pods("default").Update(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update pod labels: %w", err)
	}
	return nil
}

// UnassociatePodFromWorkspace removes the workspace-id label from a pod.
func UnassociatePodFromWorkspace(ctx context.Context, podName string) error {
	if clientset == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	pod, err := clientset.CoreV1().Pods("default").Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	if pod.Labels != nil {
		delete(pod.Labels, "workspace-id")
	}

	_, err = clientset.CoreV1().Pods("default").Update(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to remove pod labels: %w", err)
	}
	return nil
}

// CreateWorkspaceService creates a ClusterIP service for the workspace pod.
func CreateWorkspaceService(ctx context.Context, workspaceId string) (string, error) {
	if clientset == nil {
		return "", fmt.Errorf("kubernetes client not initialized")
	}

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
					Port:       8765,
					TargetPort: intstr.FromInt(8765),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	result, err := clientset.CoreV1().Services("default").Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create service: %w", err)
	}

	return result.Spec.ClusterIP, nil
}

// GetNodeExternalIP returns the external IP of one of the nodes.
func GetNodeExternalIP(ctx context.Context) (string, error) {
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
func DeleteWorkspacePod(ctx context.Context, workspaceId string) error {
	if clientset == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	// Delete the service first
	serviceName := "service-" + workspaceId
	err := clientset.CoreV1().Services("default").Delete(ctx, serviceName, metav1.DeleteOptions{})
	if err != nil {
		log.Printf("Failed to delete service %s: %v", serviceName, err)
	}

	// Find the pod by label workspace-id
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("workspace-id=%s", workspaceId),
	}
	pods, err := clientset.CoreV1().Pods("default").List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("failed to list pods for deletion: %w", err)
	}

	for _, pod := range pods.Items {
		err := clientset.CoreV1().Pods("default").Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Printf("Failed to delete pod %s: %v", pod.Name, err)
		} else {
			log.Printf("Deleted pod %s associated with workspace %s", pod.Name, workspaceId)
		}
	}

	return nil
}

// CleanupAllWorkspacePods deletes all pods with label app=shadowops.
func CleanupAllWorkspacePods(ctx context.Context) error {
	if clientset == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	listOptions := metav1.ListOptions{
		LabelSelector: "app=shadowops",
	}
	pods, err := clientset.CoreV1().Pods("default").List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	for _, pod := range pods.Items {
		err := clientset.CoreV1().Pods("default").Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Printf("Failed to delete pod %s: %v", pod.Name, err)
		} else {
			log.Printf("Deleted old workspace pod %s", pod.Name)
		}
	}
	return nil
}

// WaitForPodIP waits for the pod to be assigned an IP address.
func WaitForPodIP(ctx context.Context, podName string, timeout time.Duration) (string, error) {
	if clientset == nil {
		return "", fmt.Errorf("kubernetes client not initialized")
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pod, err := clientset.CoreV1().Pods("default").Get(ctx, podName, metav1.GetOptions{})
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
