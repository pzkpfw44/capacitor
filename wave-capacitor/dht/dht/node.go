// dht/node.go - Node identification and basic DHT node functionality
package dht

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"time"
)

// NodeID represents a unique identifier for a node in the DHT
type NodeID [20]byte

// String returns a hex string representation of the NodeID
func (n NodeID) String() string {
	return hex.EncodeToString(n[:])
}

// Distance calculates the XOR distance between two NodeIDs
func (n NodeID) Distance(other NodeID) NodeID {
	var distance NodeID
	for i := 0; i < len(n); i++ {
		distance[i] = n[i] ^ other[i]
	}
	return distance
}

// Node represents a node in the DHT network
type Node struct {
	ID         NodeID     // Unique identifier
	IP         net.IP     // IP address
	Port       int        // Port number
	PublicKey  []byte     // Ed25519 public key for authentication
	LastSeen   time.Time  // Time of last contact
	IsActive   bool       // Whether the node is considered active
	Properties Properties // Additional node properties
}

// Properties contains additional node metadata
type Properties struct {
	NodeType string            // "capacitor" or "locker"
	NumShards int              // Number of shards the node manages
	Version   string           // Software version
	Metadata  map[string]string // Additional metadata
}

// NewNode creates a new DHT node
func NewNode(ip net.IP, port int, nodeType string, numShards int) (*Node, ed25519.PrivateKey, error) {
	// Generate Ed25519 key pair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key pair: %v", err)
	}

	// Create a node ID from the public key
	var nodeID NodeID
	copy(nodeID[:], pubKey[:20]) // Use first 20 bytes of public key as node ID

	// Create the node
	node := &Node{
		ID:        nodeID,
		IP:        ip,
		Port:      port,
		PublicKey: pubKey,
		LastSeen:  time.Now(),
		IsActive:  true,
		Properties: Properties{
			NodeType:  nodeType,
			NumShards: numShards,
			Version:   "1.0.0",
			Metadata:  make(map[string]string),
		},
	}

	return node, privKey, nil
}

// NewNodeWithID creates a node with a specific ID (used for testing or when importing existing nodes)
func NewNodeWithID(id NodeID, ip net.IP, port int, nodeType string) *Node {
	return &Node{
		ID:        id,
		IP:        ip,
		Port:      port,
		PublicKey: nil, // No public key
		LastSeen:  time.Now(),
		IsActive:  true,
		Properties: Properties{
			NodeType:  nodeType,
			NumShards: 1,
			Version:   "1.0.0",
			Metadata:  make(map[string]string),
		},
	}
}

// Address returns the node's address as a string
func (n *Node) Address() string {
	return fmt.Sprintf("%s:%d", n.IP.String(), n.Port)
}

// Touch updates the node's last seen time to now
func (n *Node) Touch() {
	n.LastSeen = time.Now()
	n.IsActive = true
}

// IsExpired checks if the node has expired based on a timeout duration
func (n *Node) IsExpired(timeout time.Duration) bool {
	return time.Since(n.LastSeen) > timeout
}

// ToContact converts a Node to a Contact (for routing table)
func (n *Node) ToContact() Contact {
	return Contact{
		ID:       n.ID,
		Address:  n.Address(),
		LastSeen: n.LastSeen,
	}
}

// Contact is a lightweight version of Node used in routing tables
type Contact struct {
	ID       NodeID    // Node ID
	Address  string    // IP:Port address
	LastSeen time.Time // Time of last contact
}

// Equal checks if two node contacts are equal
func (c Contact) Equal(other Contact) bool {
	return c.ID == other.ID
}