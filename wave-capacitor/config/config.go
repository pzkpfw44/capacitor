package config

import (
	"log"
	"os"
	"strconv"
)

// Constants for directories
const (
	DataDir     = "./data"
	MessagesDir = "./data/messages"
	ContactsDir = "./data/contacts"
	KeysDir     = "./data/keys"
	CertsDir    = "./data/certs"
	ConfigDir   = "./data/config"
)

// ConfusionSalt is used for obfuscation during sharding
const ConfusionSalt = "change_this_to_a_secure_random_value_in_production"

// Config holds all configuration options for the capacitor
type Config struct {
	// Basic configuration
	Port      string
	NumShards int
	JwtSecret string

	// Database configuration
	DbHost     string
	DbPort     string
	DbUser     string
	DbPassword string
	DbName     string
	DbSslMode  string
	DbHosts    string

	// Internet connectivity
	PublicDomain string
	UseTLS       bool
	UseAutoCert  bool
	CertFile     string
	KeyFile      string

	// DHT configuration
	EnableDHT       bool
	DhtPort         int
	PublicAddress   string
	BootstrapConfig string
}

// LoadConfig sets environment variables for the DB connection, API port, and sharding configuration.
// You can override these variables when deploying.
func LoadConfig() *Config {
	cfg := &Config{
		// Basic configuration
		Port:      getEnvOrDefault("PORT", "8080"),
		NumShards: getEnvAsIntOrDefault("NUM_SHARDS", 1),
		JwtSecret: getEnvOrDefault("JWT_SECRET", "change_this_to_a_secure_random_value_in_production"),

		// Database configuration
		DbHost:     getEnvOrDefault("DB_HOST", "cockroachdb"),
		DbPort:     getEnvOrDefault("DB_PORT", "26257"),
		DbUser:     getEnvOrDefault("DB_USER", "root"),
		DbPassword: getEnvOrDefault("DB_PASSWORD", ""),
		DbName:     getEnvOrDefault("DB_NAME", "defaultdb"),
		DbSslMode:  getEnvOrDefault("DB_SSLMODE", "disable"),
		DbHosts:    getEnvOrDefault("DB_HOSTS", ""),

		// Internet connectivity
		PublicDomain: getEnvOrDefault("PUBLIC_DOMAIN", ""),
		UseTLS:       getEnvAsBoolOrDefault("USE_TLS", false),
		UseAutoCert:  getEnvAsBoolOrDefault("USE_AUTOCERT", false),
		CertFile:     getEnvOrDefault("CERT_FILE", ""),
		KeyFile:      getEnvOrDefault("KEY_FILE", ""),

		// DHT configuration
		EnableDHT:       getEnvAsBoolOrDefault("ENABLE_DHT", true),
		DhtPort:         getEnvAsIntOrDefault("DHT_PORT", 4001),
		PublicAddress:   getEnvOrDefault("PUBLIC_ADDRESS", ""),
		BootstrapConfig: getEnvOrDefault("BOOTSTRAP_CONFIG", ConfigDir+"/bootstrap.json"),
	}

	log.Println("✅ Configuration loaded")
	return cfg
}

// GetDBConnectionString builds and returns the CockroachDB connection string.
// If DB_HOSTS is set, it uses that (for multi-node clusters); otherwise, it uses DB_HOST and DB_PORT.
func (c *Config) GetDBConnectionString() string {
	if c.DbHosts != "" {
		// Use multiple hosts
		if c.DbPassword == "" {
			return "postgresql://" + c.DbUser + "@" + c.DbHosts + "/" + c.DbName + "?sslmode=" + c.DbSslMode
		}
		return "postgresql://" + c.DbUser + ":" + c.DbPassword + "@" + c.DbHosts + "/" + c.DbName + "?sslmode=" + c.DbSslMode
	}

	// Fallback to single host
	if c.DbPassword == "" {
		return "postgresql://" + c.DbUser + "@" + c.DbHost + ":" + c.DbPort + "/" + c.DbName + "?sslmode=" + c.DbSslMode
	}
	return "postgresql://" + c.DbUser + ":" + c.DbPassword + "@" + c.DbHost + ":" + c.DbPort + "/" + c.DbName + "?sslmode=" + c.DbSslMode
}

// GetJWTSecret returns the JWT secret key for token signing and verification
func (c *Config) GetJWTSecret() []byte {
	return []byte(c.JwtSecret)
}

// GetPort returns the API server port
func (c *Config) GetPort() string {
	return c.Port
}

// GetNumShards returns the number of shards configured for message storage
func (c *Config) GetNumShards() int {
	return c.NumShards
}

// EnsureDirectoriesExist creates necessary directories for the application
func EnsureDirectoriesExist() {
	dirs := []string{DataDir, MessagesDir, ContactsDir, KeysDir, CertsDir, ConfigDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("Warning: Failed to create directory %s: %v", dir, err)
		}
	}
	log.Println("✅ Required directories created")
}

// Helper functions
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsIntOrDefault(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
		log.Printf("Warning: Invalid value for %s: %s, using default: %d", key, value, defaultValue)
	}
	return defaultValue
}

func getEnvAsBoolOrDefault(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}
