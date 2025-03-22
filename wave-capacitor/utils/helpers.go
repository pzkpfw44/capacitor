package utils

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// GenerateRandomBytes generates a random byte slice of the specified length
func GenerateRandomBytes(length int) ([]byte, error) {
	bytes := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return nil, err
	}
	return bytes, nil
}

// GenerateRandomString generates a random base64 string of the specified length
func GenerateRandomString(length int) (string, error) {
	bytes, err := GenerateRandomBytes(length * 3 / 4) // Adjust for base64 encoding
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// SaveJSONToFile saves a JSON-encodable object to a file with pretty printing
func SaveJSONToFile(filePath string, data interface{}) error {
	// Create the directory if it doesn't exist
	dir := strings.TrimSuffix(filePath, "/"+strings.Split(filePath, "/")[len(strings.Split(filePath, "/"))-1])
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Marshal the data with indentation for readability
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %v", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}

// LoadJSONFromFile loads a JSON file into the provided data structure
func LoadJSONFromFile(filePath string, data interface{}) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	// Read file
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	// Unmarshal JSON
	if err := json.Unmarshal(fileData, data); err != nil {
		return fmt.Errorf("failed to unmarshal data: %v", err)
	}

	return nil
}

// Base64Encode encodes a byte slice to a base64 string
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode decodes a base64 string to a byte slice
func Base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// LogInfo logs informational messages
func LogInfo(format string, v ...interface{}) {
	log.Printf("INFO: "+format, v...)
}

// LogError logs error messages
func LogError(format string, v ...interface{}) {
	log.Printf("ERROR: "+format, v...)
}

// LogDebug logs debug messages (only when DEBUG environment variable is set)
func LogDebug(format string, v ...interface{}) {
	if os.Getenv("DEBUG") == "true" {
		log.Printf("DEBUG: "+format, v...)
	}
}

// IsProduction checks if the application is running in production mode
func IsProduction() bool {
	return os.Getenv("ENVIRONMENT") == "production"
}

// WaveSignature returns the Wave capacitor signature quote
func WaveSignature() string {
	return "Making waves in the universe, one message at a time."
}
