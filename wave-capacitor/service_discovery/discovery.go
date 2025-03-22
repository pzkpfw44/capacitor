package service_discovery

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ServiceType represents the type of service
type ServiceType string

const (
	// ServiceTypeCapacitor represents a Wave Capacitor API server
	ServiceTypeCapacitor ServiceType = "capacitor"
	
	// ServiceTypeVault represents a CockroachDB node (Vault)
	ServiceTypeVault ServiceType = "vault"
)

// ServiceInfo represents information about a discovered service
type ServiceInfo struct {
	ID         string            `json:"id"`
	Type       ServiceType       `json:"type"`
	Address    string            `json:"address"`
	Port       int               `json:"port"`
	Metadata   map[string]string `json:"metadata"`
	LastSeen   time.Time         `json:"last_seen"`
	Status     string            `json:"status"` // "online", "offline", "degraded"
	Health     float64           `json:"health"` // 0.0-1.0 health score
	Region     string            `json:"region,omitempty"`
	NumShards  int               `json:"num_shards,omitempty"`
	APIVersion string            `json:"api_version,omitempty"`
}

// ServiceDiscovery manages service discovery for Wave network
type ServiceDiscovery struct {
	services  map[string]ServiceInfo
	mu        sync.RWMutex
	selfInfo  ServiceInfo
	stopChan  chan struct{}
	registry  string // URL of the service registry (if using a centralized registry)
	isRunning bool
}

// NewServiceDiscovery creates a new service discovery instance
func NewServiceDiscovery(serviceType ServiceType, address string, port int) *ServiceDiscovery {
	hostname, _ := os.Hostname()
	
	// Generate a unique ID based on hostname, type, and address:port
	id := fmt.Sprintf("%s-%s-%s-%d", hostname, serviceType, address, port)
	
	selfInfo := ServiceInfo{
		ID:        id,
		Type:      serviceType,
		Address:   address,
		Port:      port,
		Metadata:  make(map[string]string),
		LastSeen:  time.Now(),
		Status:    "online",
		Health:    1.0,
		NumShards: getNumShardsFromEnv(),
	}
	
	return &ServiceDiscovery{
		services: make(map[string]ServiceInfo),
		selfInfo: selfInfo,
		stopChan: make(chan struct{}),
	}
}

// Start begins the service discovery process
func (sd *ServiceDiscovery) Start() error {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	
	if sd.isRunning {
		return nil // Already running
	}
	
	// Register this service
	sd.services[sd.selfInfo.ID] = sd.selfInfo
	
	// Start background service discovery
	go sd.discoverLoop()
	
	sd.isRunning = true
	log.Printf("Service discovery started for %s (%s:%d)", 
		sd.selfInfo.Type, sd.selfInfo.Address, sd.selfInfo.Port)
	
	return nil
}

// Stop halts the service discovery process
func (sd *ServiceDiscovery) Stop() {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	
	if !sd.isRunning {
		return
	}
	
	close(sd.stopChan)
	sd.isRunning = false
	log.Println("Service discovery stopped")
}

// UpdateStatus updates the status of this service
func (sd *ServiceDiscovery) UpdateStatus(status string, health float64) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	
	sd.selfInfo.Status = status
	sd.selfInfo.Health = health
	sd.selfInfo.LastSeen = time.Now()
	sd.services[sd.selfInfo.ID] = sd.selfInfo
}

// GetServices returns all discovered services of a given type
func (sd *ServiceDiscovery) GetServices(serviceType ServiceType) []ServiceInfo {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	
	var result []ServiceInfo
	for _, service := range sd.services {
		if service.Type == serviceType && isServiceActive(service) {
			result = append(result, service)
		}
	}
	
	return result
}

// GetService returns a specific service by ID
func (sd *ServiceDiscovery) GetService(id string) (ServiceInfo, bool) {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	
	service, found := sd.services[id]
	return service, found
}

// discoverLoop is the background goroutine that handles service discovery
func (sd *ServiceDiscovery) discoverLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			sd.discover()
		case <-sd.stopChan:
			return
		}
	}
}

// discover polls for services
func (sd *ServiceDiscovery) discover() {
	// In a production environment, this would use a proper service discovery mechanism
	// like Consul, Kubernetes service discovery, or a custom registry service
	
	// For now, we'll use a simple approach based on environment variables
	// WAVE_SERVICES=capacitor:192.168.1.100:8080,vault:192.168.1.101:26257
	if servicesEnv := os.Getenv("WAVE_SERVICES"); servicesEnv != "" {
		sd.discoverFromEnv(servicesEnv)
	}
	
	// If a registry URL is set, query it
	if sd.registry != "" {
		sd.discoverFromRegistry()
	}
	
	// Cleanup any services that haven't been seen in a while
	sd.cleanup()
}

// discoverFromEnv parses environment variables for service discovery
func (sd *ServiceDiscovery) discoverFromEnv(servicesEnv string) {
	servicesList := strings.Split(servicesEnv, ",")
	for _, serviceStr := range servicesList {
		parts := strings.Split(serviceStr, ":")
		if len(parts) < 3 {
			continue
		}
		
		serviceType := ServiceType(parts[0])
		address := parts[1]
		port := 0
		fmt.Sscanf(parts[2], "%d", &port)
		
		if port == 0 {
			continue
		}
		
		id := fmt.Sprintf("%s-%s-%d", serviceType, address, port)
		
		// Skip if this is our own service
		if id == sd.selfInfo.ID {
			continue
		}
		
		sd.mu.Lock()
		service, exists := sd.services[id]
		if !exists {
			service = ServiceInfo{
				ID:      id,
				Type:    serviceType,
				Address: address,
				Port:    port,
				Status:  "online",
				Health:  1.0,
			}
		}
		service.LastSeen = time.Now()
		sd.services[id] = service
		sd.mu.Unlock()
	}
}

// discoverFromRegistry queries a central registry for services
func (sd *ServiceDiscovery) discoverFromRegistry() {
	resp, err := http.Get(sd.registry + "/services")
	if err != nil {
		log.Printf("Error querying service registry: %v", err)
		return
	}
	defer resp.Body.Close()
	
	var services []ServiceInfo
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		log.Printf("Error decoding service registry response: %v", err)
		return
	}
	
	sd.mu.Lock()
	defer sd.mu.Unlock()
	
	for _, service := range services {
		// Skip if this is our own service
		if service.ID == sd.selfInfo.ID {
			continue
		}
		
		service.LastSeen = time.Now()
		sd.services[service.ID] = service
	}
}

// cleanup removes stale services that haven't been seen recently
func (sd *ServiceDiscovery) cleanup() {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	
	now := time.Now()
	for id, service := range sd.services {
		// Skip our own service
		if id == sd.selfInfo.ID {
			continue
		}
		
		// If service hasn't been seen in 5 minutes, remove it
		if now.Sub(service.LastSeen) > 5*time.Minute {
			delete(sd.services, id)
			log.Printf("Removed stale service: %s (%s)", id, service.Type)
		}
	}
}

// isServiceActive checks if a service is considered active
func isServiceActive(service ServiceInfo) bool {
	// Service is active if it's online and has been seen in the last 5 minutes
	return service.Status == "online" && 
		   time.Since(service.LastSeen) < 5*time.Minute
}

// getNumShardsFromEnv gets the number of shards from environment variables
func getNumShardsFromEnv() int {
	numShards := 1 // Default to 1 shard
	
	if shardsStr := os.Getenv("NUM_SHARDS"); shardsStr != "" {
		fmt.Sscanf(shardsStr, "%d", &numShards)
		if numShards < 1 {
			numShards = 1
		}
	}
	
	return numShards
}
