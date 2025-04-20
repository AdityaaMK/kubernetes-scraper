package graph

import (
	"fmt"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// EntityKey uniquely identifies a Kubernetes resource.
type EntityKey struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
}

// GraphNode represents a node in the relationship graph.
type GraphNode struct {
	Key        EntityKey         `json:"key"`
	Properties map[string]string `json:"properties"`
	Revision   int               `json:"revision"`
}

// GraphRelationship represents an edge/relationship in the graph.
type GraphRelationship struct {
	Source           EntityKey         `json:"source"`
	Target           EntityKey         `json:"target"`
	RelationshipType string            `json:"relationshipType"`
	Properties       map[string]string `json:"properties"`
	Revision         int               `json:"revision"`
}

// Graph holds the complete set of nodes and relationships.
type Graph struct {
	Nodes         []GraphNode         `json:"nodes"`
	Relationships []GraphRelationship `json:"relationships"`
	mu            sync.RWMutex
	revision      int
}

// NewGraph creates a new empty graph
func NewGraph() *Graph {
	return &Graph{
		Nodes:         make([]GraphNode, 0),
		Relationships: make([]GraphRelationship, 0),
		revision:      1,
	}
}

// AddNode adds a node to the graph
func (g *Graph) AddNode(obj interface{}) {
	g.mu.Lock()
	defer g.mu.Unlock()

	node := objectToGraphNode(obj)
	if node == nil {
		return
	}

	// Check if node already exists
	for i, n := range g.Nodes {
		if n.Key.Name == node.Key.Name && n.Key.Namespace == node.Key.Namespace && n.Key.Type == node.Key.Type {
			g.Nodes[i] = *node
			g.revision++
			return
		}
	}

	// Add new node
	g.Nodes = append(g.Nodes, *node)
	g.revision++
}

// UpdateNode updates an existing node in the graph
func (g *Graph) UpdateNode(obj interface{}) {
	g.AddNode(obj) // AddNode handles both adding and updating
}

// RemoveNode removes a node from the graph
func (g *Graph) RemoveNode(obj interface{}) {
	g.mu.Lock()
	defer g.mu.Unlock()

	node := objectToGraphNode(obj)
	if node == nil {
		return
	}

	// Remove node
	for i, n := range g.Nodes {
		if n.Key.Name == node.Key.Name && n.Key.Namespace == node.Key.Namespace && n.Key.Type == node.Key.Type {
			g.Nodes = append(g.Nodes[:i], g.Nodes[i+1:]...)
			g.revision++
			return
		}
	}

	// Remove relationships involving this node
	for i := 0; i < len(g.Relationships); i++ {
		rel := g.Relationships[i]
		if (rel.Source.Name == node.Key.Name && rel.Source.Namespace == node.Key.Namespace && rel.Source.Type == node.Key.Type) ||
			(rel.Target.Name == node.Key.Name && rel.Target.Namespace == node.Key.Namespace && rel.Target.Type == node.Key.Type) {
			g.Relationships = append(g.Relationships[:i], g.Relationships[i+1:]...)
			i--
		}
	}
}

// objectToGraphNode converts a Kubernetes object to a GraphNode
func objectToGraphNode(obj interface{}) *GraphNode {
	var key EntityKey
	var properties map[string]string

	switch o := obj.(type) {
	case *corev1.Pod:
		key = EntityKey{
			Name:      o.Name,
			Namespace: o.Namespace,
			Type:      "Pod",
		}
		properties = map[string]string{
			"status": string(o.Status.Phase),
		}
	case *appsv1.ReplicaSet:
		key = EntityKey{
			Name:      o.Name,
			Namespace: o.Namespace,
			Type:      "ReplicaSet",
		}
		properties = map[string]string{
			"replicas": fmt.Sprintf("%d", *o.Spec.Replicas),
		}
	case *appsv1.Deployment:
		key = EntityKey{
			Name:      o.Name,
			Namespace: o.Namespace,
			Type:      "Deployment",
		}
		properties = map[string]string{
			"replicas": fmt.Sprintf("%d", *o.Spec.Replicas),
		}
	case *corev1.Node:
		key = EntityKey{
			Name:      o.Name,
			Namespace: "",
			Type:      "Node",
		}
		properties = map[string]string{
			"status": string(o.Status.Phase),
		}
	case *corev1.Service:
		key = EntityKey{
			Name:      o.Name,
			Namespace: o.Namespace,
			Type:      "Service",
		}
		properties = map[string]string{
			"type": string(o.Spec.Type),
		}
	case *corev1.ConfigMap:
		key = EntityKey{
			Name:      o.Name,
			Namespace: o.Namespace,
			Type:      "ConfigMap",
		}
		properties = map[string]string{
			"data": fmt.Sprintf("%v", o.Data),
		}
	default:
		return nil
	}

	return &GraphNode{
		Key:        key,
		Properties: properties,
		Revision:   1,
	}
}

// AddRelationship adds a relationship to the graph
func (g *Graph) AddRelationship(source, target EntityKey, relationshipType string, properties map[string]string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Check if relationship already exists
	for i, rel := range g.Relationships {
		if rel.Source.Name == source.Name && rel.Source.Namespace == source.Namespace && rel.Source.Type == source.Type &&
			rel.Target.Name == target.Name && rel.Target.Namespace == target.Namespace && rel.Target.Type == target.Type &&
			rel.RelationshipType == relationshipType {
			g.Relationships[i].Properties = properties
			g.Relationships[i].Revision++
			g.revision++
			return
		}
	}

	// Add new relationship
	g.Relationships = append(g.Relationships, GraphRelationship{
		Source:           source,
		Target:           target,
		RelationshipType: relationshipType,
		Properties:       properties,
		Revision:         1,
	})
	g.revision++
}

// RemoveRelationship removes a relationship from the graph
func (g *Graph) RemoveRelationship(source, target EntityKey, relationshipType string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for i, rel := range g.Relationships {
		if rel.Source.Name == source.Name && rel.Source.Namespace == source.Namespace && rel.Source.Type == source.Type &&
			rel.Target.Name == target.Name && rel.Target.Namespace == target.Namespace && rel.Target.Type == target.Type &&
			rel.RelationshipType == relationshipType {
			g.Relationships = append(g.Relationships[:i], g.Relationships[i+1:]...)
			g.revision++
			return
		}
	}
}
