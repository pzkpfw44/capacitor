// dht/routing.go - Kademlia routing table implementation
package dht

import (
	"container/list"
	"sort"
	"sync"
	"time"
)

const (
	// K is the size of a k-bucket in the Kademlia routing table
	K = 20

	// Alpha is the concurrency parameter for network calls
	Alpha = 3

	// RefreshInterval is how often to refresh buckets
	RefreshInterval = 1 * time.Hour

	// ReplicationInterval is how often to replicate data
	ReplicationInterval = 1 * time.Hour

	// ExpireTime is how long a node can be inactive before considered offline
	ExpireTime = 24 * time.Hour
)

// KBucket represents a Kademlia k-bucket in the routing table
type KBucket struct {
	mutex    sync.RWMutex
	contacts *list.List    // Ordered list of contacts
	lastSeen time.Time     // Last time this bucket was updated
	range    struct {       // Range of node IDs in this bucket
		min, max NodeID
	}
}

// NewKBucket creates a new k-bucket
func NewKBucket() *KBucket {
	kb := &KBucket{
		contacts: list.New(),
		lastSeen: time.Now(),
	}
	// Initialize min to all 1s and max to all 0s (will be replaced)
	for i := 0; i < 20; i++ {
		kb.range.min[i] = 0xFF
		kb.range.max[i] = 0x00
	}
	return kb
}

// AddContact adds or updates a contact in the k-bucket
func (kb *KBucket) AddContact(contact Contact) bool {
	kb.mutex.Lock()
	defer kb.mutex.Unlock()

	// Check if contact already exists
	for e := kb.contacts.Front(); e != nil; e = e.Next() {
		if existingContact := e.Value.(Contact); existingContact.Equal(contact) {
			// Move to the end (most recently seen)
			kb.contacts.MoveToBack(e)
			// Update last seen
			e.Value = contact
			kb.lastSeen = time.Now()
			return true
		}
	}

	// If the bucket isn't full, add the contact
	if kb.contacts.Len() < K {
		kb.contacts.PushBack(contact)
		kb.lastSeen = time.Now()
		// Update ID range for the bucket
		if lessThan(contact.ID, kb.range.min) {
			kb.range.min = contact.ID
		}
		if lessThan(kb.range.max, contact.ID) {
			kb.range.max = contact.ID
		}
		return true
	}

	// The bucket is full - check if the least recently used contact is still active
	// In a real implementation, you would ping the node here
	// For now, we'll just not add the contact if the bucket is full
	return false
}

// GetContacts returns up to 'count' contacts from the k-bucket
func (kb *KBucket) GetContacts(count int) []Contact {
	kb.mutex.RLock()
	defer kb.mutex.RUnlock()

	// If count is greater than bucket size, limit it
	if count > kb.contacts.Len() {
		count = kb.contacts.Len()
	}

	contacts := make([]Contact, 0, count)
	for e := kb.contacts.Front(); e != nil && len(contacts) < count; e = e.Next() {
		contacts = append(contacts, e.Value.(Contact))
	}
	return contacts
}

// Size returns the number of contacts in the k-bucket
func (kb *KBucket) Size() int {
	kb.mutex.RLock()
	defer kb.mutex.RUnlock()
	return kb.contacts.Len()
}

// RoutingTable implements a Kademlia routing table
type RoutingTable struct {
	mutex   sync.RWMutex
	localID NodeID
	buckets []*KBucket
}

// NewRoutingTable creates a new routing table
func NewRoutingTable(localID NodeID) *RoutingTable {
	rt := &RoutingTable{
		localID: localID,
		buckets: make([]*KBucket, 160), // 160 bits = 20 bytes
	}

	// Initialize all buckets
	for i := 0; i < 160; i++ {
		rt.buckets[i] = NewKBucket()
	}

	return rt
}

// AddContact adds a contact to the routing table
func (rt *RoutingTable) AddContact(contact Contact) {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()

	// Calculate the bucket index based on the XOR distance
	bucketIndex := rt.getBucketIndex(contact.ID)
	rt.buckets[bucketIndex].AddContact(contact)
}

// GetClosestContacts returns the k closest contacts to the given node ID
func (rt *RoutingTable) GetClosestContacts(target NodeID, count int) []Contact {
	rt.mutex.RLock()
	defer rt.mutex.RUnlock()

	bucketIndex := rt.getBucketIndex(target)
	
	// First, check the target bucket
	contacts := rt.buckets[bucketIndex].GetContacts(count)
	
	// If we need more contacts, check neighboring buckets
	for i := 1; len(contacts) < count && (bucketIndex-i >= 0 || bucketIndex+i < 160); i++ {
		// Check bucket to the left
		if bucketIndex-i >= 0 {
			contacts = append(contacts, rt.buckets[bucketIndex-i].GetContacts(count-len(contacts))...)
		}
		
		// Check bucket to the right
		if bucketIndex+i < 160 && len(contacts) < count {
			contacts = append(contacts, rt.buckets[bucketIndex+i].GetContacts(count-len(contacts))...)
		}
	}

	// Sort contacts by distance to target
	sort.Slice(contacts, func(i, j int) bool {
		distI := contacts[i].ID.Distance(target)
		distJ := contacts[j].ID.Distance(target)
		return lessThan(distI, distJ)
	})

	// Limit to count
	if len(contacts) > count {
		contacts = contacts[:count]
	}

	return contacts
}

// GetBucketIndex finds the index of the bucket that would contain the given node ID
func (rt *RoutingTable) getBucketIndex(id NodeID) int {
	distance := rt.localID.Distance(id)
	
	// Find the index of the first bit that is 1 in the distance
	for i := 0; i < len(distance); i++ {
		for j := 0; j < 8; j++ {
			if (distance[i] >> (7 - j)) & 0x1 != 0 {
				return i*8 + j
			}
		}
	}
	
	// If all bits are 0 (same ID), use the last bucket
	return 159
}

// GetRandomIDFromBucket generates a random ID that would fall into the given bucket
func (rt *RoutingTable) GetRandomIDFromBucket(bucketIndex int) NodeID {
	// Starting with our own ID, flip the bit at bucketIndex
	var id NodeID
	copy(id[:], rt.localID[:])
	
	byteIndex := bucketIndex / 8
	bitIndex := bucketIndex % 8
	
	// Flip the specific bit
	id[byteIndex] ^= byte(1 << (7 - bitIndex))
	
	return id
}

// Size returns the total number of contacts in the routing table
func (rt *RoutingTable) Size() int {
	rt.mutex.RLock()
	defer rt.mutex.RUnlock()
	
	total := 0
	for _, bucket := range rt.buckets {
		total += bucket.Size()
	}
	return total
}

// lessThan compares two NodeIDs lexicographically
func lessThan(a, b NodeID) bool {
	for i := 0; i < len(a); i++ {
		if a[i] < b[i] {
			return true
		}
		if a[i] > b[i] {
			return false
		}
	}
	return false
}