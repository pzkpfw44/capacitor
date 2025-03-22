package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
	"wave_capacitor/config"
	"wave_capacitor/middleware"
	"wave_capacitor/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// SendMessageRequest defines the structure for sending message requests
type SendMessageRequest struct {
	RecipientPublicKey  string `json:"recipient_pubkey"`
	CiphertextKEM       string `json:"ciphertext_kem"`
	CiphertextMsg       string `json:"ciphertext_msg"`
	Nonce               string `json:"nonce"`
	SenderCiphertextKEM string `json:"sender_ciphertext_kem"`
	SenderCiphertextMsg string `json:"sender_ciphertext_msg"`
	SenderNonce         string `json:"sender_nonce"`
}

// Message represents the structure of a stored message
type Message struct {
	MessageID           string    `json:"message_id"`
	SenderPublicKey     string    `json:"sender_public_key"`
	RecipientPublicKey  string    `json:"recipient_public_key"`
	CiphertextKEM       string    `json:"ciphertext_kem"`
	CiphertextMsg       string    `json:"ciphertext_msg"`
	Nonce               string    `json:"nonce"`
	SenderCiphertextKEM string    `json:"sender_ciphertext_kem,omitempty"`
	SenderCiphertextMsg string    `json:"sender_ciphertext_msg,omitempty"`
	SenderNonce         string    `json:"sender_nonce,omitempty"`
	Timestamp           time.Time `json:"timestamp"`
}

// GetMessageFolder calculates the folder path for a user's messages based on their public key
// This implements the obfuscation layer using a hash with a confusion salt
func GetMessageFolder(publicKey string) string {
	// Combine public key with confusion salt
	data := publicKey + config.ConfusionSalt
	hash := sha256.Sum256([]byte(data))
	hashStr := hex.EncodeToString(hash[:])

	// Get the configured number of shards
	numShards := config.GetNumShards()
	
	if numShards <= 1 {
		// If no sharding, just use the first 16 chars of the hash
		return filepath.Join(config.MessagesDir, hashStr[:16])
	} else {
		// With sharding, calculate shard index based on the first byte of the hash
		shardIndex := int(hash[0]) % numShards
		folderName := fmt.Sprintf("%s_%d", hashStr[:16], shardIndex)
		return filepath.Join(config.MessagesDir, folderName)
	}
}

// SendMessage handles storing an encrypted message for both sender and recipient
func SendMessage(c *fiber.Ctx) error {
	// Parse request body
	var req SendMessageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request format",
		})
	}

	// Validate required fields
	if req.RecipientPublicKey == "" || req.CiphertextKEM == "" || 
	   req.CiphertextMsg == "" || req.Nonce == "" ||
	   req.SenderCiphertextKEM == "" || req.SenderCiphertextMsg == "" || 
	   req.SenderNonce == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Missing required message fields",
		})
	}

	// Get sender username from JWT
	username := middleware.ExtractUsername(c)

	// Get sender's public key from database
	user, err := models.GetUser(username)
	if err != nil {
		log.Printf("Error retrieving sender user: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to retrieve sender information",
		})
	}
	senderPublicKey := user.PublicKey

	// Generate message ID and timestamp
	messageID := uuid.New().String()
	timestamp := time.Now()

	// Create message object
	message := Message{
		MessageID:           messageID,
		SenderPublicKey:     senderPublicKey,
		RecipientPublicKey:  req.RecipientPublicKey,
		CiphertextKEM:       req.CiphertextKEM,
		CiphertextMsg:       req.CiphertextMsg,
		Nonce:               req.Nonce,
		SenderCiphertextKEM: req.SenderCiphertextKEM,
		SenderCiphertextMsg: req.SenderCiphertextMsg,
		SenderNonce:         req.SenderNonce,
		Timestamp:           timestamp,
	}

	// Marshal message to JSON
	messageJSON, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to process message",
		})
	}

	// Store message for recipient
	recipientFolder := GetMessageFolder(req.RecipientPublicKey)
	if err := os.MkdirAll(recipientFolder, 0755); err != nil {
		log.Printf("Error creating recipient folder: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to store message for recipient",
		})
	}
	recipientFilePath := filepath.Join(recipientFolder, messageID+".json")
	if err := ioutil.WriteFile(recipientFilePath, messageJSON, 0644); err != nil {
		log.Printf("Error writing recipient message: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to store message for recipient",
		})
	}

	// Store a copy for sender
	senderFolder := GetMessageFolder(senderPublicKey)
	if err := os.MkdirAll(senderFolder, 0755); err != nil {
		log.Printf("Error creating sender folder: %v", err)
		// Continue anyway as the message is already stored for the recipient
	} else {
		senderFilePath := filepath.Join(senderFolder, messageID+".json")
		if err := ioutil.WriteFile(senderFilePath, messageJSON, 0644); err != nil {
			log.Printf("Error writing sender message: %v", err)
			// Continue anyway as the message is already stored for the recipient
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":    true,
		"message":    "Message sent successfully",
		"message_id": messageID,
		"timestamp":  timestamp,
	})
}

// GetMessages retrieves all messages for the authenticated user
func GetMessages(c *fiber.Ctx) error {
	// Get username from JWT
	username := middleware.ExtractUsername(c)

	// Get user's public key from database
	user, err := models.GetUser(username)
	if err != nil {
		log.Printf("Error retrieving user for messages: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to retrieve user information",
		})
	}

	// Calculate the user's message folder
	folder := GetMessageFolder(user.PublicKey)
	
	// Check if folder exists
	if _, err := os.Stat(folder); os.IsNotExist(err) {
		// Return empty messages array if folder doesn't exist
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success":  true,
			"messages": []Message{},
		})
	}

	// Read message files from folder
	files, err := ioutil.ReadDir(folder)
	if err != nil {
		log.Printf("Error reading message directory: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to retrieve messages",
		})
	}

	// Process each message file
	messages := []Message{}
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue // Skip non-JSON files
		}

		// Read message file
		filePath := filepath.Join(folder, file.Name())
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Printf("Error reading message file %s: %v", file.Name(), err)
			continue // Skip this file and try the next one
		}

		// Unmarshal message
		var message Message
		if err := json.Unmarshal(data, &message); err != nil {
			log.Printf("Error unmarshaling message %s: %v", file.Name(), err)
			continue // Skip this file and try the next one
		}

		// Add message to array
		messages = append(messages, message)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":  true,
		"messages": messages,
	})
}
