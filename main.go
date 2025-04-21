package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/AdityaaMK/kubernetes-scraper/graph"
	"github.com/AdityaaMK/kubernetes-scraper/k8sclient"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

// Global maps to track pods and services for dynamic relationship updates
var (
	podCache     = make(map[string]map[string]interface{})
	serviceCache = make(map[string]map[string]interface{})
	cacheMutex   = sync.RWMutex{}
)

func main() {
	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create Kubernetes client
	client, err := k8sclient.NewK8sClient()
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
	}

	// Create graph
	g := graph.NewGraph()

	// List all resources
	if err := listAllResources(ctx, client, g); err != nil {
		log.Printf("Error listing resources: %v", err)
	}

	// Watch all resources
	go watchAllResources(ctx, client, g)

	// Emit graph periodically
	go emitGraph(ctx, g)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
}

func listAllResources(ctx context.Context, client *k8sclient.K8sClient, g *graph.Graph) error {
	// List Pods
	pods, err := client.ListPods(ctx)
	if err != nil {
		return fmt.Errorf("error listing pods: %v", err)
	}
	cacheMutex.Lock()
	for _, pod := range pods {
		podObj := pod.(map[string]interface{})
		podName := podObj["metadata"].(map[string]interface{})["name"].(string)
		podNamespace := podObj["metadata"].(map[string]interface{})["namespace"].(string)
		podCache[fmt.Sprintf("%s/%s", podNamespace, podName)] = podObj
	}
	cacheMutex.Unlock()
	for _, pod := range pods {
		g.AddNode(pod)
	}

	// List ReplicaSets
	replicasets, err := client.ListReplicaSets(ctx)
	if err != nil {
		return fmt.Errorf("error listing replicasets: %v", err)
	}
	for _, rs := range replicasets {
		g.AddNode(rs)
	}

	// List Deployments
	deployments, err := client.ListDeployments(ctx)
	if err != nil {
		return fmt.Errorf("error listing deployments: %v", err)
	}
	for _, deployment := range deployments {
		g.AddNode(deployment)
	}

	// List Nodes
	nodes, err := client.ListNodes(ctx)
	if err != nil {
		return fmt.Errorf("error listing nodes: %v", err)
	}
	for _, node := range nodes {
		g.AddNode(node)
	}

	// List Services
	services, err := client.ListServices(ctx)
	if err != nil {
		return fmt.Errorf("error listing services: %v", err)
	}
	cacheMutex.Lock()
	for _, service := range services {
		serviceObj := service.(map[string]interface{})
		serviceName := serviceObj["metadata"].(map[string]interface{})["name"].(string)
		serviceNamespace := serviceObj["metadata"].(map[string]interface{})["namespace"].(string)
		serviceCache[fmt.Sprintf("%s/%s", serviceNamespace, serviceName)] = serviceObj
	}
	cacheMutex.Unlock()
	for _, service := range services {
		g.AddNode(service)
	}

	// List ConfigMaps
	configmaps, err := client.ListConfigMaps(ctx)
	if err != nil {
		return fmt.Errorf("error listing configmaps: %v", err)
	}
	for _, configmap := range configmaps {
		g.AddNode(configmap)
	}

	// Create relationships
	for _, pod := range pods {
		podObj := pod.(map[string]interface{})
		podName := podObj["metadata"].(map[string]interface{})["name"].(string)
		podNamespace := podObj["metadata"].(map[string]interface{})["namespace"].(string)
		nodeName := podObj["spec"].(map[string]interface{})["nodeName"].(string)

		// Get ownerReferences with nil check
		ownerRefs := []interface{}{}
		if ownerRefsInterface, ok := podObj["metadata"].(map[string]interface{})["ownerReferences"]; ok && ownerRefsInterface != nil {
			ownerRefs = ownerRefsInterface.([]interface{})
		}

		// Pod -> Node relationship
		if nodeName != "" {
			g.AddRelationship(
				graph.EntityKey{Name: podName, Namespace: podNamespace, Type: "Pod"},
				graph.EntityKey{Name: nodeName, Type: "Node"},
				"runs_on",
				nil,
			)
		}

		// Pod -> ReplicaSet relationship
		for _, ownerRef := range ownerRefs {
			owner := ownerRef.(map[string]interface{})
			if owner["kind"].(string) == "ReplicaSet" {
				g.AddRelationship(
					graph.EntityKey{Name: podName, Namespace: podNamespace, Type: "Pod"},
					graph.EntityKey{Name: owner["name"].(string), Namespace: podNamespace, Type: "ReplicaSet"},
					"owned_by",
					nil,
				)
			}
		}
	}

	for _, rs := range replicasets {
		rsObj := rs.(map[string]interface{})
		rsName := rsObj["metadata"].(map[string]interface{})["name"].(string)
		rsNamespace := rsObj["metadata"].(map[string]interface{})["namespace"].(string)
		ownerRefs := rsObj["metadata"].(map[string]interface{})["ownerReferences"].([]interface{})

		// ReplicaSet -> Deployment relationship
		for _, ownerRef := range ownerRefs {
			owner := ownerRef.(map[string]interface{})
			if owner["kind"].(string) == "Deployment" {
				g.AddRelationship(
					graph.EntityKey{Name: rsName, Namespace: rsNamespace, Type: "ReplicaSet"},
					graph.EntityKey{Name: owner["name"].(string), Namespace: rsNamespace, Type: "Deployment"},
					"owned_by",
					nil,
				)
			}
		}
	}

	for _, service := range services {
		serviceObj := service.(map[string]interface{})
		serviceName := serviceObj["metadata"].(map[string]interface{})["name"].(string)
		serviceNamespace := serviceObj["metadata"].(map[string]interface{})["namespace"].(string)

		// Get service selector with nil check
		selector := make(map[string]interface{})
		if selectorInterface, ok := serviceObj["spec"].(map[string]interface{})["selector"]; ok && selectorInterface != nil {
			selector = selectorInterface.(map[string]interface{})
		}

		// Service -> Pod relationships based on selector
		for _, pod := range pods {
			podObj := pod.(map[string]interface{})
			podName := podObj["metadata"].(map[string]interface{})["name"].(string)
			podNamespace := podObj["metadata"].(map[string]interface{})["namespace"].(string)

			// Get pod labels with nil check
			podLabels := make(map[string]interface{})
			if labelsInterface, ok := podObj["metadata"].(map[string]interface{})["labels"]; ok && labelsInterface != nil {
				podLabels = labelsInterface.(map[string]interface{})
			}

			// Check if pod labels match service selector
			matches := true
			for key, value := range selector {
				if podLabels[key] != value {
					matches = false
					break
				}
			}

			if matches {
				g.AddRelationship(
					graph.EntityKey{Name: serviceName, Namespace: serviceNamespace, Type: "Service"},
					graph.EntityKey{Name: podName, Namespace: podNamespace, Type: "Pod"},
					"targets",
					nil,
				)
			}
		}
	}

	for _, deployment := range deployments {
		deploymentObj := deployment.(map[string]interface{})
		deploymentName := deploymentObj["metadata"].(map[string]interface{})["name"].(string)
		deploymentNamespace := deploymentObj["metadata"].(map[string]interface{})["namespace"].(string)
		volumes := deploymentObj["spec"].(map[string]interface{})["template"].(map[string]interface{})["spec"].(map[string]interface{})["volumes"].([]interface{})

		// Deployment -> ConfigMap relationships
		for _, volume := range volumes {
			vol := volume.(map[string]interface{})
			if configMap, ok := vol["configMap"]; ok {
				configMapName := configMap.(map[string]interface{})["name"].(string)
				g.AddRelationship(
					graph.EntityKey{Name: deploymentName, Namespace: deploymentNamespace, Type: "Deployment"},
					graph.EntityKey{Name: configMapName, Namespace: deploymentNamespace, Type: "ConfigMap"},
					"uses",
					nil,
				)
			}
		}
	}

	return nil
}

func watchAllResources(ctx context.Context, client *k8sclient.K8sClient, g *graph.Graph) {
	// Watch Pods
	go watchResource(ctx, client.WatchPods, g, "Pod")

	// Watch ReplicaSets
	go watchResource(ctx, client.WatchReplicaSets, g, "ReplicaSet")

	// Watch Deployments
	go watchResource(ctx, client.WatchDeployments, g, "Deployment")

	// Watch Nodes
	go watchResource(ctx, client.WatchNodes, g, "Node")

	// Watch Services
	go watchResource(ctx, client.WatchServices, g, "Service")

	// Watch ConfigMaps
	go watchResource(ctx, client.WatchConfigMaps, g, "ConfigMap")
}

func watchResource(ctx context.Context, watchFunc func(context.Context) (watch.Interface, error), g *graph.Graph, resourceType string) {
	for {
		watcher, err := watchFunc(ctx)
		if err != nil {
			log.Printf("Error watching %s: %v", resourceType, err)
			time.Sleep(5 * time.Second)
			continue
		}

		for {
			select {
			case <-ctx.Done():
				watcher.Stop()
				return
			case event, ok := <-watcher.ResultChan():
				if !ok {
					log.Printf("%s watcher closed", resourceType)
					return
				}

				// Convert runtime.Object to unstructured.Unstructured
				unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(event.Object)
				if err != nil {
					log.Printf("Error converting object to unstructured: %v", err)
					continue
				}

				metadata := unstructuredObj["metadata"].(map[string]interface{})
				name := metadata["name"].(string)

				// Get namespace with nil check
				namespace := ""
				if namespaceInterface, ok := metadata["namespace"]; ok && namespaceInterface != nil {
					namespace = namespaceInterface.(string)
				}

				switch event.Type {
				case watch.Added:
					log.Printf("%s added: %v", resourceType, name)
					g.AddNode(event.Object)
					updateRelationships(g, unstructuredObj, resourceType, name, namespace)
					if resourceType == "Pod" {
						cacheMutex.Lock()
						podCache[fmt.Sprintf("%s/%s", namespace, name)] = unstructuredObj
						cacheMutex.Unlock()
					} else if resourceType == "Service" {
						cacheMutex.Lock()
						serviceCache[fmt.Sprintf("%s/%s", namespace, name)] = unstructuredObj
						cacheMutex.Unlock()
					}
				case watch.Modified:
					log.Printf("%s modified: %v", resourceType, name)
					g.UpdateNode(event.Object)
					updateRelationships(g, unstructuredObj, resourceType, name, namespace)
					if resourceType == "Pod" {
						cacheMutex.Lock()
						podCache[fmt.Sprintf("%s/%s", namespace, name)] = unstructuredObj
						cacheMutex.Unlock()
					} else if resourceType == "Service" {
						cacheMutex.Lock()
						serviceCache[fmt.Sprintf("%s/%s", namespace, name)] = unstructuredObj
						cacheMutex.Unlock()
					}
				case watch.Deleted:
					log.Printf("%s deleted: %v", resourceType, name)
					g.RemoveNode(event.Object)
					// Remove all relationships involving this resource
					removeResourceRelationships(g, graph.EntityKey{Name: name, Namespace: namespace, Type: resourceType})
					if resourceType == "Pod" {
						cacheMutex.Lock()
						delete(podCache, fmt.Sprintf("%s/%s", namespace, name))
						cacheMutex.Unlock()
					} else if resourceType == "Service" {
						cacheMutex.Lock()
						delete(serviceCache, fmt.Sprintf("%s/%s", namespace, name))
						cacheMutex.Unlock()
					}
				}
			}
		}
	}
}

func updateRelationships(g *graph.Graph, obj map[string]interface{}, resourceType, name, namespace string) {
	switch resourceType {
	case "Pod":
		// Pod -> Node relationship
		if nodeName, ok := obj["spec"].(map[string]interface{})["nodeName"].(string); ok && nodeName != "" {
			g.AddRelationship(
				graph.EntityKey{Name: name, Namespace: namespace, Type: "Pod"},
				graph.EntityKey{Name: nodeName, Type: "Node"},
				"runs_on",
				nil,
			)
		}

		// Pod -> ReplicaSet relationship
		if ownerRefs, ok := obj["metadata"].(map[string]interface{})["ownerReferences"].([]interface{}); ok {
			for _, ownerRef := range ownerRefs {
				owner := ownerRef.(map[string]interface{})
				if owner["kind"].(string) == "ReplicaSet" {
					g.AddRelationship(
						graph.EntityKey{Name: name, Namespace: namespace, Type: "Pod"},
						graph.EntityKey{Name: owner["name"].(string), Namespace: namespace, Type: "ReplicaSet"},
						"owned_by",
						nil,
					)
				}
			}
		}

	case "ReplicaSet":
		// ReplicaSet -> Deployment relationship
		if ownerRefs, ok := obj["metadata"].(map[string]interface{})["ownerReferences"].([]interface{}); ok {
			for _, ownerRef := range ownerRefs {
				owner := ownerRef.(map[string]interface{})
				if owner["kind"].(string) == "Deployment" {
					g.AddRelationship(
						graph.EntityKey{Name: name, Namespace: namespace, Type: "ReplicaSet"},
						graph.EntityKey{Name: owner["name"].(string), Namespace: namespace, Type: "Deployment"},
						"owned_by",
						nil,
					)
				}
			}
		}

	case "Service":
		// Service -> Pod relationships based on selector
		if selector, ok := obj["spec"].(map[string]interface{})["selector"].(map[string]interface{}); ok {
			// Get all current pods and update relationships with this service
			cacheMutex.RLock()
			for _, pod := range podCache {
				podName := pod["metadata"].(map[string]interface{})["name"].(string)
				podNamespace := pod["metadata"].(map[string]interface{})["namespace"].(string)

				// Get pod labels with nil check
				podLabels := make(map[string]interface{})
				if labelsInterface, ok := pod["metadata"].(map[string]interface{})["labels"]; ok && labelsInterface != nil {
					podLabels = labelsInterface.(map[string]interface{})
				}

				// Check if pod labels match service selector
				matches := true
				for key, value := range selector {
					if podLabels[key] != value {
						matches = false
						break
					}
				}

				if matches {
					g.AddRelationship(
						graph.EntityKey{Name: name, Namespace: namespace, Type: "Service"},
						graph.EntityKey{Name: podName, Namespace: podNamespace, Type: "Pod"},
						"targets",
						nil,
					)
				} else {
					// Remove relationship if it exists but no longer matches
					g.RemoveRelationship(
						graph.EntityKey{Name: name, Namespace: namespace, Type: "Service"},
						graph.EntityKey{Name: podName, Namespace: podNamespace, Type: "Pod"},
						"targets",
					)
				}
			}
			cacheMutex.RUnlock()
		}

	case "Deployment":
		// Deployment -> ConfigMap relationships
		if volumes, ok := obj["spec"].(map[string]interface{})["template"].(map[string]interface{})["spec"].(map[string]interface{})["volumes"].([]interface{}); ok {
			for _, volume := range volumes {
				vol := volume.(map[string]interface{})
				if configMap, ok := vol["configMap"]; ok {
					configMapName := configMap.(map[string]interface{})["name"].(string)
					g.AddRelationship(
						graph.EntityKey{Name: name, Namespace: namespace, Type: "Deployment"},
						graph.EntityKey{Name: configMapName, Namespace: namespace, Type: "ConfigMap"},
						"uses",
						nil,
					)
				}
			}
		}
	}
}

func removeResourceRelationships(g *graph.Graph, key graph.EntityKey) {
	// Remove all relationships where this resource is either the source or target
	for _, rel := range g.Relationships {
		if (rel.Source.Name == key.Name && rel.Source.Namespace == key.Namespace && rel.Source.Type == key.Type) ||
			(rel.Target.Name == key.Name && rel.Target.Namespace == key.Namespace && rel.Target.Type == key.Type) {
			g.RemoveRelationship(rel.Source, rel.Target, rel.RelationshipType)
		}
	}
}

func emitGraph(ctx context.Context, g *graph.Graph) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			data, err := json.MarshalIndent(g, "", "  ")
			if err != nil {
				log.Printf("Error marshaling graph: %v", err)
				continue
			}

			if err := os.WriteFile("graph.json", data, 0644); err != nil {
				log.Printf("Error writing graph: %v", err)
			}
		}
	}
}
