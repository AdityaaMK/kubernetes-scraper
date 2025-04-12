package main

import (
	"context"
	"log"
	"time"

	"github.com/AdityaaMK/kubernetes-scraper/k8sclient"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"encoding/json"

	"os"

	"github.com/AdityaaMK/kubernetes-scraper/graph"
)

func watchPods(clientset *k8sclient.ClientsetWrapper, g *graph.Graph) {
	watcher, err := clientset.CoreV1().Pods("").Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Error watching pods: %v", err)
	}

	// Process watch events
	go func() {
		for event := range watcher.ResultChan() {
			// event.Type could be ADDED, MODIFIED, DELETED
			log.Printf("Pod event: %v", event.Type)

			// Convert event.Object to Pod
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				log.Printf("Error converting event object to Pod")
				continue
			}

			// Add or update pod in graph based on event type
			node := k8sclient.PodToGraphNode(pod)
			switch event.Type {
			case watch.Added, watch.Modified:
				// Add or update node
				found := false
				for i, n := range g.Nodes {
					if n.Key.Name == node.Key.Name && n.Key.Namespace == node.Key.Namespace {
						g.Nodes[i] = node
						found = true
						break
					}
				}
				if !found {
					g.Nodes = append(g.Nodes, node)
				}
			case watch.Deleted:
				// Remove node
				for i, n := range g.Nodes {
					if n.Key.Name == node.Key.Name && n.Key.Namespace == node.Key.Namespace {
						g.Nodes = append(g.Nodes[:i], g.Nodes[i+1:]...)
						break
					}
				}
			}
		}
	}()
}

func emitGraph(g *graph.Graph, filename string) {
	file, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Unable to create file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // prettify the JSON output
	if err := encoder.Encode(g); err != nil {
		log.Fatalf("Error encoding graph to JSON: %v", err)
	}
	log.Printf("Graph successfully written to %s", filename)
}

func listPods(clientset *k8sclient.ClientsetWrapper, g *graph.Graph) {
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Error listing pods: %v", err)
	}
	for _, pod := range pods.Items {
		log.Printf("Found pod: %s in namespace %s", pod.Name, pod.Namespace)
		// Add pod to graph
		node := k8sclient.PodToGraphNode(&pod)
		g.Nodes = append(g.Nodes, node)
	}
}

func main() {
	// Step 1: Create Kubernetes clientset.
	clientset := k8sclient.GetClientset()

	// Step 2: Initialize an empty graph.
	var g graph.Graph

	// Step 3: Initial listing (for example, list pods and add to graph).
	listPods(clientset, &g)
	// Repeat listing for other resources and update 'g' accordingly.

	// Step 4: Start watching for events in separate goroutines.
	go watchPods(clientset, &g)
	// Start watchers for ReplicaSets, Deployments, Nodes, Services, ConfigMaps.

	// Step 5: Periodically emit the graph to JSON.
	ticker := time.NewTicker(30 * time.Second)
	for {
		select {
		case <-ticker.C:
			emitGraph(&g, "graph.json")
		}
	}
}
