package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// Configuration values that should be imported from your config package
// If these aren't defined in your config package, you'll need to add them
var (
	// GetNumShards should be defined in your config package
	// If not available, you can define it directly in this file
	GetNumShards = func() int {
		// Default to 1 if not configured elsewhere
		return 1
	}

	// ConfusionSalt should be defined in your config package
	// If not available, define it here
	ConfusionSalt = "my_super_secret_salt" // This should match your config value
)

// ShardManager handles the logic for distributing data across multiple shards
type ShardManager struct {
	numShards     int
	confusionSalt string
	baseDir       string
}

// NewShardManager creates a new instance of the ShardManager
func NewShardManager(baseDir string) *ShardManager {
	return &ShardManager{
		numShards:     GetNumShards(),
		confusionSalt: ConfusionSalt,
		baseDir:       baseDir,
	}
}

// GetShardIndexForKey calculates which shard a particular key belongs to
func (sm *ShardManager) GetShardIndexForKey(key string) int {
	if sm.numShards <= 1 {
		return 0
	}

	// Mix the key with the confusion salt and hash it
	data := key + sm.confusionSalt
	hash := sha256.Sum256([]byte(data))

	// Use the first byte of the hash to determine the shard
	return int(hash[0]) % sm.numShards
}

// GetFolderForKey returns the folder path for storing data associated with a key
func (sm *ShardManager) GetFolderForKey(key string) string {
	// Hash the key with the confusion salt
	data := key + sm.confusionSalt
	hash := sha256.Sum256([]byte(data))
	hashPrefix := hex.EncodeToString(hash[:])[:16]

	if sm.numShards <= 1 {
		// No sharding, just use the hash prefix
		return filepath.Join(sm.baseDir, hashPrefix)
	}

	// With sharding, include the shard index in the folder name
	shardIndex := sm.GetShardIndexForKey(key)
	folderName := fmt.Sprintf("%s_%d", hashPrefix, shardIndex)
	return filepath.Join(sm.baseDir, folderName)
}

// GetAllShards returns paths to all possible shard folders
func (sm *ShardManager) GetAllShards() []string {
	if sm.numShards <= 1 {
		return []string{sm.baseDir}
	}

	shards := make([]string, sm.numShards)
	for i := 0; i < sm.numShards; i++ {
		shards[i] = filepath.Join(sm.baseDir, fmt.Sprintf("shard_%d", i))
	}
	return shards
}

// DistributeData returns the appropriate folder for the data based on its key
// This is a wrapper around GetFolderForKey that ensures the folder exists
func (sm *ShardManager) DistributeData(key string) (string, error) {
	folder := sm.GetFolderForKey(key)

	// Ensure the folder exists
	if err := EnsureDirectoryExists(folder); err != nil {
		return "", fmt.Errorf("failed to create shard directory: %v", err)
	}

	return folder, nil
}

// EnsureDirectoryExists creates a directory if it doesn't exist
// If this function is defined in your config package, you should import and use that instead
func EnsureDirectoryExists(dir string) error {
	return os.MkdirAll(dir, 0755)
}
