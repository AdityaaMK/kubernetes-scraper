package k8sclient

import (
	"github.com/AdityaaMK/kubernetes-scraper/graph"

	corev1 "k8s.io/api/core/v1"
)

// PodToGraphNode converts a Pod object to a GraphNode.
func PodToGraphNode(pod *corev1.Pod) graph.GraphNode {
	props := map[string]string{
		"status": string(pod.Status.Phase),
		// Add other key fields as needed.
	}
	return graph.GraphNode{
		Key: graph.EntityKey{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Type:      "Pod",
		},
		Properties: props,
		Revision:   int(pod.Generation), // or any revision indicator
	}
}
