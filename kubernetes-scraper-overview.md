# Overview of the Kubernetes Relationship Scraper

## How the Codebase Works

The Kubernetes relationship scraper (or "Satellite") extracts resources from a Kubernetes cluster, builds a graph of their relationships, and outputs this graph as JSON. Here's the architecture:

### Components

1. **Main Package**: Orchestrates the entire process

   - Handles initialization and lifecycle management
   - Sets up resource watching
   - Manages caches for dynamic relationship tracking
   - Periodically emits the graph as JSON

2. **Graph Package**: Manages the relationship data structure

   - Defines `EntityKey` for uniquely identifying resources
   - Provides methods for adding/removing nodes and relationships
   - Supports JSON serialization of the graph

3. **K8sClient Package**: Interfaces with the Kubernetes API
   - Handles authentication to the cluster
   - Provides methods to list and watch resources
   - Converts Kubernetes objects to a consistent format

### Core Workflow

1. **Initialization**:

   - Connect to the Kubernetes cluster (using in-cluster config or kubeconfig)
   - Initialize the graph data structure
   - Set up caches for pods and services

2. **Initial Resource Discovery**:

   - List all required resources (Pods, ReplicaSets, Deployments, Nodes, Services, ConfigMaps)
   - Add each resource as a node in the graph
   - Store pods and services in caches for relationship tracking

3. **Relationship Mapping**:

   - Map Pod → Node relationships (which node a pod runs on)
   - Map Pod → ReplicaSet relationships (ownership)
   - Map ReplicaSet → Deployment relationships (ownership)
   - Map Service → Pod relationships (via label selectors)
   - Map Deployment → ConfigMap relationships (volume mounts)

4. **Dynamic Updates**:

   - Start watchers for each resource type
   - When resources are added/modified/deleted, update the graph
   - Use caches to efficiently recalculate relationships when pods or services change

5. **Graph Emission**:
   - Periodically (every 30 seconds) emit the graph as JSON
   - The graph includes all resources as nodes and their relationships as edges

### Key Relationship Types

| Source     | Target     | Relationship Type | Mechanism                  |
| ---------- | ---------- | ----------------- | -------------------------- |
| Pod        | Node       | runs_on           | Pod's nodeName             |
| Pod        | ReplicaSet | owned_by          | OwnerReferences            |
| ReplicaSet | Deployment | owned_by          | OwnerReferences            |
| Service    | Pod        | targets           | Label selector matching    |
| Deployment | ConfigMap  | uses              | Volume mount configuration |

### Concurrency Management

- Uses goroutines for parallel processing (one per resource type)
- Employs read/write mutexes to protect shared state (graph and caches)
- Ensures thread-safe updates to the relationship graph
- Handles graceful shutdown via context cancellation

## How to Demo the Scraper

### Setup

1. **Start Minikube**:

   ```bash
   minikube start
   ```

2. **Build the Scraper**:

   ```bash
   cd /path/to/kubernetes-scraper
   go build -o kubernetes-scraper
   ```

3. **Run the Scraper**:
   ```bash
   ./kubernetes-scraper
   ```

### Demo Steps

1. **Show Initial Graph**:
   After 30 seconds, the scraper will generate a `graph.json` file.

   ```bash
   cat graph.json | jq
   ```

   This shows all existing resources in your Minikube cluster and their relationships.

2. **Deploy Sample Resources**:

   ```bash
   # Create a ConfigMap
   kubectl create configmap demo-config --from-literal=key1=value1

   # Create a Deployment using the ConfigMap
   kubectl create deployment nginx --image=nginx
   kubectl patch deployment nginx --type=json \
     -p='[{"op": "add", "path": "/spec/template/spec/volumes", "value": [{"name": "config-volume", "configMap": {"name": "demo-config"}}]}]'

   # Create a Service
   kubectl expose deployment nginx --port=80
   ```

3. **Show Graph Updates**:
   After 30 seconds, view the updated graph:

   ```bash
   cat graph.json | jq
   ```

   Point out the new resources and relationships that appeared:

   - The nginx Deployment
   - ReplicaSets created by the Deployment
   - Pods created by the ReplicaSet
   - Service targeting the Pods
   - ConfigMap used by the Deployment

4. **Demonstrate Dynamic Relationships**:

   ```bash
   # Scale the deployment
   kubectl scale deployment nginx --replicas=3

   # Wait 30 seconds and check the graph
   cat graph.json | jq
   ```

   Show how the new Pods automatically connect to the Service.

5. **Show Label Selector Relationship Updates**:

   ```bash
   # Change a Pod label to break its relationship with the Service
   POD_NAME=$(kubectl get pods -l app=nginx -o jsonpath='{.items[0].metadata.name}')
   kubectl label pod $POD_NAME app=modified --overwrite

   # Check the graph after 30 seconds
   cat graph.json | jq
   ```

   Show how the Service→Pod relationship for that Pod is now removed.

## Resource Efficiency and API Server Considerations

### Lightweight Design

1. **Minimal Memory Footprint**:

   - Only stores essential metadata about resources, not full specs
   - Uses efficient graph data structure optimized for relationship tracking
   - Implements caching only for resources that need dynamic relationship updates (pods and services)

2. **Efficient CPU Utilization**:

   - Event-driven architecture that only processes changes
   - Only recalculates affected relationships when resources change
   - Avoids expensive graph traversals or recomputation of unchanged relationships

3. **Minimal Disk Usage**:
   - Outputs compact JSON that only includes necessary information
   - Stateless operation with no database dependencies
   - Small binary size due to Go's static compilation

### Kubernetes API Server Considerations

1. **Watch-Based Updates**:

   - Uses Kubernetes watch API instead of repeated polling
   - Receives streaming updates rather than repeatedly listing resources
   - Significantly reduces load on the API server compared to periodic listing

2. **Single Initial Load**:

   - Only performs one full listing of resources at startup
   - All subsequent updates come through the watch API
   - Minimizes the number of API requests

3. **Error Handling and Backoff**:

   - Implements retry with backoff for transient API errors
   - Reconnects watches gracefully when they disconnect
   - Avoids hammering the API server during outages

4. **Efficient Relationship Calculation**:

   - Performs relationship mapping in memory using local caches
   - No additional API calls needed to establish relationships
   - Leverages OwnerReferences metadata when available

5. **Label Selector Processing**:
   - Processes Service→Pod label selector matching locally
   - No additional API queries to find matching Pods
   - Maintains accuracy by updating when either Pods or Services change

These design choices make the scraper very lightweight and considerate of Kubernetes API server resources, making it suitable for continuous monitoring of even large clusters without causing performance issues.
