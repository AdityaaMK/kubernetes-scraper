package k8sclient

import (
	"context"
	"log"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// K8sClient wraps the kubernetes clientset
type K8sClient struct {
	clientset *kubernetes.Clientset
}

// NewK8sClient creates a new Kubernetes client
func NewK8sClient() (*K8sClient, error) {
	var config *rest.Config
	var err error

	// Try to get in-cluster config first
	config, err = rest.InClusterConfig()
	if err != nil {
		// If not in cluster, try to get local config
		home := homedir.HomeDir()
		if home == "" {
			log.Fatal("Could not find home directory")
		}
		kubeconfig := filepath.Join(home, ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &K8sClient{
		clientset: clientset,
	}, nil
}

// ListPods lists all pods in all namespaces
func (c *K8sClient) ListPods(ctx context.Context) ([]interface{}, error) {
	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Convert to []interface{}
	result := make([]interface{}, len(pods.Items))
	for i, pod := range pods.Items {
		result[i] = pod
	}
	return result, nil
}

// ListReplicaSets lists all replicasets in all namespaces
func (c *K8sClient) ListReplicaSets(ctx context.Context) ([]interface{}, error) {
	replicasets, err := c.clientset.AppsV1().ReplicaSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, len(replicasets.Items))
	for i, rs := range replicasets.Items {
		result[i] = rs
	}
	return result, nil
}

// ListDeployments lists all deployments in all namespaces
func (c *K8sClient) ListDeployments(ctx context.Context) ([]interface{}, error) {
	deployments, err := c.clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, len(deployments.Items))
	for i, deployment := range deployments.Items {
		result[i] = deployment
	}
	return result, nil
}

// ListNodes lists all nodes
func (c *K8sClient) ListNodes(ctx context.Context) ([]interface{}, error) {
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, len(nodes.Items))
	for i, node := range nodes.Items {
		result[i] = node
	}
	return result, nil
}

// ListServices lists all services in all namespaces
func (c *K8sClient) ListServices(ctx context.Context) ([]interface{}, error) {
	services, err := c.clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, len(services.Items))
	for i, service := range services.Items {
		result[i] = service
	}
	return result, nil
}

// ListConfigMaps lists all configmaps in all namespaces
func (c *K8sClient) ListConfigMaps(ctx context.Context) ([]interface{}, error) {
	configmaps, err := c.clientset.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, len(configmaps.Items))
	for i, configmap := range configmaps.Items {
		result[i] = configmap
	}
	return result, nil
}

// WatchPods watches for pod events
func (c *K8sClient) WatchPods(ctx context.Context) (watch.Interface, error) {
	return c.clientset.CoreV1().Pods("").Watch(ctx, metav1.ListOptions{})
}

// WatchReplicaSets watches for replicaset events
func (c *K8sClient) WatchReplicaSets(ctx context.Context) (watch.Interface, error) {
	return c.clientset.AppsV1().ReplicaSets("").Watch(ctx, metav1.ListOptions{})
}

// WatchDeployments watches for deployment events
func (c *K8sClient) WatchDeployments(ctx context.Context) (watch.Interface, error) {
	return c.clientset.AppsV1().Deployments("").Watch(ctx, metav1.ListOptions{})
}

// WatchNodes watches for node events
func (c *K8sClient) WatchNodes(ctx context.Context) (watch.Interface, error) {
	return c.clientset.CoreV1().Nodes().Watch(ctx, metav1.ListOptions{})
}

// WatchServices watches for service events
func (c *K8sClient) WatchServices(ctx context.Context) (watch.Interface, error) {
	return c.clientset.CoreV1().Services("").Watch(ctx, metav1.ListOptions{})
}

// WatchConfigMaps watches for configmap events
func (c *K8sClient) WatchConfigMaps(ctx context.Context) (watch.Interface, error) {
	return c.clientset.CoreV1().ConfigMaps("").Watch(ctx, metav1.ListOptions{})
}
