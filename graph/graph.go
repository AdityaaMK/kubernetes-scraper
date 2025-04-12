package graph

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
}
