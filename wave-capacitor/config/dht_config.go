// config/dht_config.go - DHT configuration management for Wave Capacitor
package config

import (
	"os"
	"strings"
	"time"
	"strconv"
)

// DHTConfig contains configuration settings for the DHT
type DHTConfig struct {
	// Network Configuration
	ListenAddress  string        // Address to listen on for DHT communication
	ExternalIP     string        // External IP address for others to contact us
	DHTPort        int           // Port for DHT communication
	APIPort        int           // Port for API communication
	GRPCPort       int           // Port for gRPC communication
	
	// Discovery Configuration
	BootstrapNodes []string      // List of seed nodes for bootstrapping
	RefreshInterval time.Duration // How often to refresh routing table
	
	// Node Configuration
	NumShards      int           // Number of shards this node manages
	NodeID         string        // Optional override for Node ID
	
	// Storage Configuration
	StoragePath    string        // Path for DHT data storage
	
	// Security Configuration
	UseSSL         bool          // Whether to use SSL for DHT communication
	CertFile       string        // Path to SSL certificate
	KeyFile        string        // Path to SSL key
}

// LoadDHTConfig loads DHT configuration from environment variables
// with Capacitor-specific defaults
func LoadDHTConfig() *DHTConfig {
	cfg := &DHTConfig{
		// Set default values appropriate for Capacitor
		ListenAddress:   getEnvOrDefault("DHT_LISTEN_ADDRESS", "0.0.0.0"),
		ExternalIP:      getEnvOrDefault("DHT_EXTERNAL_IP", ""),
		DHTPort:         getEnvAsIntOrDefault("DHT_PORT", 4000),        // Default Capacitor DHT port
		APIPort:         getEnvAsIntOrDefault("API_PORT", 8080),        // Default Capacitor API port
		GRPCPort:        getEnvAsIntOrDefault("GRPC_PORT", 9090),
		BootstrapNodes:  parseBootstrapNodes(getEnvOrDefault("DHT_BOOTSTRAP_NODES", "")),
		RefreshInterval: time.Duration(getEnvAsIntOrDefault("DHT_REFRESH_INTERVAL_MINUTES", 60)) * time.Minute,
		NumShards:       getEnvAsIntOrDefault("NUM_SHARDS", 1),         // Default shards for Capacitor
		NodeID:          getEnvOrDefault("DHT_NODE_ID", ""),
		StoragePath:     getEnvOrDefault("DHT_STORAGE_PATH", "./data/dht"),
		UseSSL:          getEnvAsBoolOrDefault("DHT_USE_SSL", false),
		CertFile:        getEnvOrDefault("DHT_CERT_FILE", ""),
		KeyFile:         getEnvOrDefault("DHT_KEY_FILE", ""),
	}
	
	// If external IP is not specified, try to determine it
	if cfg.ExternalIP == "" {
		// In a production environment, you would use a service like stun.healthchecks.io
		// or implement a function to determine the external IP
		cfg.ExternalIP = cfg.ListenAddress
		if cfg.ListenAddress == "0.0.0.0" {
			cfg.ExternalIP = "localhost" // Default fallback
		}
	}
	
	return cfg
}

// parseBootstrapNodes parses a comma-separated list of bootstrap nodes
func parseBootstrapNodes(nodesStr string) []string {
	if nodesStr == "" {
		return []string{}
	}
	
	// Split the string by commas
	nodes := strings.Split(nodesStr, ",")
	
	// Trim whitespace and filter empty entries
	var result []string
	for _, node := range nodes {
		node = strings.TrimSpace(node)
		if node != "" {
			result = append(result, node)
		}
	}
	
	return result
}

// GetDHTAddress returns the full DHT listen address (IP:Port)
func (c *DHTConfig) GetDHTAddress() string {
	return c.ListenAddress + ":" + strconv.Itoa(c.DHTPort)
}

// GetExternalDHTAddress returns the external DHT address (IP:Port)
func (c *DHTConfig) GetExternalDHTAddress() string {
	return c.ExternalIP + ":" + strconv.Itoa(c.DHTPort)
}

// MakeDHTStorageDirectory creates the DHT storage directory if it doesn't exist
func (c *DHTConfig) MakeDHTStorageDirectory() error {
	return os.MkdirAll(c.StoragePath, 0755)
}

// AddBootstrapNode adds a bootstrap node to the configuration
func (c *DHTConfig) AddBootstrapNode(node string) {
	// Check if node is already in the list
	for _, n := range c.BootstrapNodes {
		if n == node {
			return
		}
	}
	
	c.BootstrapNodes = append(c.BootstrapNodes, node)
}

// ClearBootstrapNodes removes all bootstrap nodes
func (c *DHTConfig) ClearBootstrapNodes() {
	c.BootstrapNodes = []string{}
}

// getEnvAsBoolOrDefault gets an environment variable as bool with a default fallback
func getEnvAsBoolOrDefault(key string, defaultVal bool) bool {
	if val, exists := os.LookupEnv(key); exists {
		boolVal, err := strconv.ParseBool(val)
		if err == nil {
			return boolVal
		}
	}
	return defaultVal
}