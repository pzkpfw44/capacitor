package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"wave_capacitor/middleware"
	"wave_capacitor/models"

	"github.com/gofiber/fiber/v2"
)

// BackupData represents the structure of a complete account backup
type BackupData struct {
	Username            string                 `json:"username"`
	PublicKey           string                 `json:"public_key"`
	EncryptedPrivateKey interface{}            `json:"encrypted_private_key"`
	Contacts            map[string]interface{} `json:"contacts"`
	Messages            []interface{}          `json:"messages"`
}

// RecoverRequest defines the structure for account recovery requests
type RecoverRequest struct {
	Username            string                 `json:"username"`
	PublicKey           string                 `json:"public_key"`
	EncryptedPrivateKey interface{}            `json:"encrypted_private_key"`
	Contacts            map[string]interface{} `json:"contacts"`
	Messages            []interface{}          `json:"messages"`
}

// BackupAccount handles creating a complete backup of a user's account data
func BackupAccount(c *fiber.Ctx) error {
	// Get username from JWT
	username := middleware.ExtractUsername(c)

	// Get user data from database
	user, err := models.GetUser(username)
	if err != nil {
		log.Printf("Error retrieving user for backup: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to retrieve user information",
		})
	}

	// Load contacts
	contacts := make(map[string]interface{})
	contactsFile := filepath.Join("./data/contacts", username+".json")
	if _, err := os.Stat(contactsFile); err == nil {
		data, err := ioutil.ReadFile(contactsFile)
		if err != nil {
			log.Printf("Error reading contacts file: %v", err)
		} else {
			json.Unmarshal(data, &contacts)
		}
	}

	// Load messages
	messages := []interface{}{}
	messageFolder := GetMessageFolder(user.PublicKey)
	if _, err := os.Stat(messageFolder); err == nil {
		files, err := ioutil.ReadDir(messageFolder)
		if err != nil {
			log.Printf("Error reading messages folder: %v", err)
		} else {
			for _, file := range files {
				if filepath.Ext(file.Name()) != ".json" {
					continue
				}

				path := filepath.Join(messageFolder, file.Name())
				data, err := ioutil.ReadFile(path)
				if err != nil {
					log.Printf("Error reading message file %s: %v", file.Name(), err)
					continue
				}

				var msg interface{}
				if err := json.Unmarshal(data, &msg); err != nil {
					log.Printf("Error unmarshaling message %s: %v", file.Name(), err)
					continue
				}

				messages = append(messages, msg)
			}
		}
	}

	// Create backup data
	backupData := BackupData{
		Username:            username,
		PublicKey:           user.PublicKey,
		EncryptedPrivateKey: user.EncryptedPrivKey,
		Contacts:            contacts,
		Messages:            messages,
	}

	return c.Status(fiber.StatusOK).JSON(backupData)
}

// RecoverAccount handles restoring an account from a backup
func RecoverAccount(c *fiber.Ctx) error {
	// Parse request body
	var req RecoverRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request format",
		})
	}

	// Validate required fields
	if req.Username == "" || req.PublicKey == "" || req.EncryptedPrivateKey == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Username, public key, and encrypted private key are required",
		})
	}

	// Update user keys in database
	err := models.UpdateUserKeys(req.Username, req.PublicKey, req.EncryptedPrivateKey)
	if err != nil {
		log.Printf("Error updating user keys: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to update user keys",
		})
	}

	// Restore contacts if provided
	if req.Contacts != nil && len(req.Contacts) > 0 {
		contactsFile := filepath.Join("./data/contacts", req.Username+".json")
		os.MkdirAll("./data/contacts", 0755)

		contactsData, err := json.MarshalIndent(req.Contacts, "", "  ")
		if err != nil {
			log.Printf("Error marshaling contacts data: %v", err)
		} else {
			if err := ioutil.WriteFile(contactsFile, contactsData, 0644); err != nil {
				log.Printf("Error writing contacts file: %v", err)
			}
		}
	}

	// Restore messages if provided
	if req.Messages != nil && len(req.Messages) > 0 {
		messageFolder := GetMessageFolder(req.PublicKey)
		os.MkdirAll(messageFolder, 0755)

		for i, msgData := range req.Messages {
			// Generate a message ID if not present
			msgMap, ok := msgData.(map[string]interface{})
			if !ok {
				log.Printf("Invalid message data format at index %d", i)
				continue
			}

			msgID, ok := msgMap["message_id"].(string)
			if !ok || msgID == "" {
				msgID = fmt.Sprintf("recovered_%d", i)
				msgMap["message_id"] = msgID
			}

			messageData, err := json.MarshalIndent(msgMap, "", "  ")
			if err != nil {
				log.Printf("Error marshaling message data: %v", err)
				continue
			}

			messagePath := filepath.Join(messageFolder, msgID+".json")
			if err := ioutil.WriteFile(messagePath, messageData, 0644); err != nil {
				log.Printf("Error writing message file: %v", err)
			}
		}
	}

	// Generate JWT token for the recovered account
	token, err := middleware.GenerateToken(req.Username)
	if err != nil {
		log.Printf("Error generating token for recovered account: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to generate authentication token",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Account recovered successfully",
		"token":   token,
	})
}
