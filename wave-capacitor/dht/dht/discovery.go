// dht/discovery.go - Service discovery and node lookup implementation
package dht

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// ServiceInfo contains information about a service in the DHT
type ServiceInfo struct {
	NodeID     NodeID            `json:"node_id"`
	NodeType   string            `json:"node_type"`
	Address    string            `json:"address"`
	APIPort    int               `json:"api_port"`
	GRPCPort   int               `json:"grpc_port"`
	NumShards  int               `json:"num_shards"`
	Version    string            `json:"version"`
	Properties map[string]string `json:"properties"`
	LastSeen   time.Time         `json:"last_seen"`
}

// DHT represents the main Distributed Hash Table implementation
type DHT struct {
	mutex       sync.RWMutex
	localNode   *Node
	routingTable *RoutingTable
	services     map[string]ServiceInfo // Services by service ID
	privateKey   []byte                 // Node's private key
	config       *DHTConfig             // DHT configuration
	httpClient   *http.Client           // HTTP client for node communication
	server       *http.Server           // HTTP server for node API
	shutdown     chan struct{}          // Channel to signal shutdown
	wg           sync.WaitGroup         // Wait group for background tasks
}

// DHTConfig contains configuration for the DHT
type DHTConfig struct {
	BootstrapNodes  []string      // List of initial bootstrap nodes
	ListenAddr      string        // Address to listen on (IP:Port)
	APIPort         int           // Port for REST API
	GRPCPort        int           // Port for gRPC API
	RefreshInterval time.Duration // How often to refresh routing table
	NodeType        string        // "capacitor" or "locker"
	NumShards       int           // Number of shards for this node
	StoreDir        string        // Directory to store DHT data
}

// NewDHT creates a new DHT instance
func NewDHT(cfg *DHTConfig) (*DHT, error) {
	// Parse listen address
	host, _, err := net.SplitHostPort(cfg.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid listen address: %v", err)
	}
	
	ip := net.ParseIP(host)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", host)
	}
	
	// Create the local node
	node, privateKey, err := NewNode(ip, cfg.APIPort, cfg.NodeType, cfg.NumShards)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %v", err)
	}
	
	// Initialize DHT
	dht := &DHT{
		localNode:    node,
		routingTable: NewRoutingTable(node.ID),
		services:     make(map[string]ServiceInfo),
		privateKey:   privateKey,
		config:       cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		shutdown: make(chan struct{}),
	}
	
	return dht, nil
}

// Start begins the DHT operations
func (dht *DHT) Start() error {
	// Start the HTTP server for node communication
	if err := dht.startServer(); err != nil {
		return err
	}
	
	// Start background tasks
	dht.wg.Add(3)
	go dht.refreshRoutingTable()
	go dht.republishServices()
	go dht.expireContacts()
	
	// Bootstrap the DHT
	return dht.bootstrap()
}

// Stop gracefully shuts down the DHT
func (dht *DHT) Stop() error {
	// Signal all background tasks to stop
	close(dht.shutdown)
	
	// Shutdown the HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := dht.server.Shutdown(ctx); err != nil {
		return err
	}
	
	// Wait for all background tasks to complete
	dht.wg.Wait()
	
	return nil
}

// bootstrap connects to initial nodes and populates the routing table
func (dht *DHT) bootstrap() error {
	if len(dht.config.BootstrapNodes) == 0 {
		// No bootstrap nodes, we're the first node
		return nil
	}
	
	// Connect to bootstrap nodes
	for _, addr := range dht.config.BootstrapNodes {
		if err := dht.addBootstrapNode(addr); err != nil {
			// Log the error but continue with other nodes
			fmt.Printf("Failed to add bootstrap node %s: %v\n", addr, err)
		}
	}
	
	// Perform node lookup for our own ID to populate routing table
	return dht.FindNode(dht.localNode.ID)
}

// addBootstrapNode adds a single bootstrap node to the routing table
func (dht *DHT) addBootstrapNode(addr string) error {
	// Create a temporary contact for the bootstrap node
	// We'll get the real NodeID when we connect
	var tempID NodeID
	contact := Contact{
		ID:      tempID, // Will be replaced with real ID
		Address: addr,
		LastSeen: time.Now(),
	}
	
	// Try to ping the bootstrap node
	nodeInfo, err := dht.pingNode(contact)
	if err != nil {
		return err
	}
	
	// Create a proper contact with the real NodeID
	realContact := Contact{
		ID:      nodeInfo.NodeID,
		Address: addr,
		LastSeen: time.Now(),
	}
	
	// Add to routing table
	dht.routingTable.AddContact(realContact)
	
	return nil
}

// FindNode performs a Kademlia FIND_NODE operation
func (dht *DHT) FindNode(targetID NodeID) error {
	// Get alpha closest nodes from routing table
	closestNodes := dht.routingTable.GetClosestContacts(targetID, Alpha)
	if len(closestNodes) == 0 {
		return fmt.Errorf("no contacts in routing table")
	}
	
	// Keep track of nodes we've already contacted
	contacted := make(map[string]bool)
	for _, contact := range closestNodes {
		contacted[contact.Address] = true
	}
	
	// Use a channel to collect results from parallel lookups
	resultChan := make(chan []Contact, Alpha)
	
	// Query the alpha closest nodes in parallel
	activeQueries := 0
	for _, contact := range closestNodes {
		activeQueries++
		go func(c Contact) {
			contacts, err := dht.findNodeRPC(c, targetID)
			if err != nil {
				resultChan <- nil
				return
			}
			resultChan <- contacts
		}(contact)
	}
	
	// Process results and continue querying nodes
	var closestSoFar []Contact
	for activeQueries > 0 {
		select {
		case contacts := <-resultChan:
			activeQueries--
			
			if contacts == nil {
				continue
			}
			
			// Add new contacts to our list
			for _, contact := range contacts {
				// Skip contacts we've already seen
				if contacted[contact.Address] {
					continue
				}
				
				contacted[contact.Address] = true
				closestSoFar = append(closestSoFar, contact)
				
				// Add to routing table
				dht.routingTable.AddContact(contact)
			}
			
			// Sort by distance to target
			sort.Slice(closestSoFar, func(i, j int) bool {
				distI := closestSoFar[i].ID.Distance(targetID)
				distJ := closestSoFar[j].ID.Distance(targetID)
				return lessThan(distI, distJ)
			})
			
			// If we have more contacts to query, start a new query
			if len(closestSoFar) > 0 && activeQueries < Alpha {
				next := closestSoFar[0]
				closestSoFar = closestSoFar[1:]
				
				activeQueries++
				go func(c Contact) {
					contacts, err := dht.findNodeRPC(c, targetID)
					if err != nil {
						resultChan <- nil
						return
					}
					resultChan <- contacts
				}(next)
			}
		}
	}
	
	return nil
}

// findNodeRPC performs a FIND_NODE RPC call to another node
func (dht *DHT) findNodeRPC(contact Contact, targetID NodeID) ([]Contact, error) {
	url := fmt.Sprintf("http://%s/dht/findnode", contact.Address)
	
	// Create the request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	// Add query parameters
	q := req.URL.Query()
	q.Add("target", targetID.String())
	req.URL.RawQuery = q.Encode()
	
	// Send the request
	resp, err := dht.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	// Parse the response
	var result struct {
		Contacts []Contact `json:"contacts"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return result.Contacts, nil
}

// pingNode pings a node to get its information
func (dht *DHT) pingNode(contact Contact) (*ServiceInfo, error) {
	url := fmt.Sprintf("http://%s/dht/ping", contact.Address)
	
	// Send the request
	resp, err := dht.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	// Parse the response
	var info ServiceInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	
	return &info, nil
}

// startServer starts the HTTP server for DHT communication
func (dht *DHT) startServer() error {
	mux := http.NewServeMux()
	
	// Add DHT endpoints
	mux.HandleFunc("/dht/ping", dht.handlePing)
	mux.HandleFunc("/dht/findnode", dht.handleFindNode)
	mux.HandleFunc("/dht/findvalue", dht.handleFindValue)
	mux.HandleFunc("/dht/store", dht.handleStore)
	
	// Create server
	dht.server = &http.Server{
		Addr:    dht.config.ListenAddr,
		Handler: mux,
	}
	
	// Start server in a goroutine
	go func() {
		if err := dht.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()
	
	return nil
}

// Handler for /dht/ping
func (dht *DHT) handlePing(w http.ResponseWriter, r *http.Request) {
	// Return node info
	info := ServiceInfo{
		NodeID:    dht.localNode.ID,
		NodeType:  dht.localNode.Properties.NodeType,
		Address:   dht.localNode.Address(),
		APIPort:   dht.config.APIPort,
		GRPCPort:  dht.config.GRPCPort,
		NumShards: dht.localNode.Properties.NumShards,
		Version:   dht.localNode.Properties.Version,
		Properties: dht.localNode.Properties.Metadata,
		LastSeen:  time.Now(),
	}
	
	// Update the caller in our routing table
	// In a real implementation, we would extract the caller's NodeID from the request
	
	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// Handler for /dht/findnode
func (dht *DHT) handleFindNode(w http.ResponseWriter, r *http.Request) {
	// Get target ID from query parameter
	targetStr := r.URL.Query().Get("target")
	if targetStr == "" {
		http.Error(w, "Missing target parameter", http.StatusBadRequest)
		return
	}
	
	// Parse target ID
	var targetID NodeID
	if n, err := hex.Decode(targetID[:], []byte(targetStr)); err != nil || n != 20 {
		http.Error(w, "Invalid target ID", http.StatusBadRequest)
		return
	}
	
	// Find k closest nodes
	closestContacts := dht.routingTable.GetClosestContacts(targetID, K)
	
	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"contacts": closestContacts,
	})
}

// Handler for /dht/findvalue (stub)
func (dht *DHT) handleFindValue(w http.ResponseWriter, r *http.Request) {
	// This would be implemented for a full DHT
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// Handler for /dht/store (stub)
func (dht *DHT) handleStore(w http.ResponseWriter, r *http.Request) {
	// This would be implemented for a full DHT
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// Background tasks

// refreshRoutingTable periodically refreshes the routing table
func (dht *DHT) refreshRoutingTable() {
	defer dht.wg.Done()
	
	ticker := time.NewTicker(dht.config.RefreshInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Refresh random bucket
			bucketIndex := rand.Intn(160)
			randomID := dht.routingTable.GetRandomIDFromBucket(bucketIndex)
			dht.FindNode(randomID)
			
		case <-dht.shutdown:
			return
		}
	}
}

// republishServices periodically republishes services
func (dht *DHT) republishServices() {
	defer dht.wg.Done()
	
	ticker := time.NewTicker(ReplicationInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// This would republish stored services
			
		case <-dht.shutdown:
			return
		}
	}
}

// expireContacts periodically expires old contacts
func (dht *DHT) expireContacts() {
	defer dht.wg.Done()
	
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// This would expire old contacts
			
		case <-dht.shutdown:
			return
		}
	}
}

// RegisterService registers a service in the DHT
func (dht *DHT) RegisterService(serviceID string, info ServiceInfo) error {
	dht.mutex.Lock()
	defer dht.mutex.Unlock()
	
	// Store service locally
	dht.services[serviceID] = info
	
	// In a full implementation, we would also store the service in the DHT
	
	return nil
}

// FindService looks up a service by ID
func (dht *DHT) FindService(serviceID string) (*ServiceInfo, error) {
	dht.mutex.RLock()
	defer dht.mutex.RUnlock()
	
	// Check if we have it locally
	if info, ok := dht.services[serviceID]; ok {
		return &info, nil
	}
	
	// In a full implementation, we would look up the service in the DHT
	
	return nil, fmt.Errorf("service not found")
}

// FindServicesByType finds services by type
func (dht *DHT) FindServicesByType(serviceType string) ([]ServiceInfo, error) {
	dht.mutex.RLock()
	defer dht.mutex.RUnlock()
	
	var result []ServiceInfo
	
	// Check local services
	for _, info := range dht.services {
		if info.NodeType == serviceType {
			result = append(result, info)
		}
	}
	
	// In a full implementation, we would also search the DHT
	
	return result, nil
}