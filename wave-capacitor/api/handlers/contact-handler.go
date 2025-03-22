package handlers

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"wave_capacitor/config"
	"wave_capacitor/middleware"

	"github.com/gofiber/fiber/v2"
)

// Contact represents a contact entry
type Contact struct {
	PublicKey string `json:"public_key"`
	Nickname  string `json:"nickname"`
}

// ContactsData represents the structure of contacts storage
type ContactsData map[string]Contact

// AddContactRequest defines the structure for adding a contact
type AddContactRequest struct {
	ContactPublicKey string `json:"contact_public_key"`
	Nickname         string `json:"nickname"`
}

// RemoveContactRequest defines the structure for removing a contact
type RemoveContactRequest struct {
	ContactPublicKey string `json:"contact_public_key"`
}

// getContactsFile returns the path to a user's contacts file
func getContactsFile(username string) string {
	return filepath.Join(config.ContactsDir, username+".json")
}

// loadContacts loads contacts from a user's contacts file
func loadContacts(username string) (ContactsData, error) {
	contactsFile := getContactsFile(username)
	contacts := make(ContactsData)

	// Check if file exists
	if _, err := os.Stat(contactsFile); os.IsNotExist(err) {
		return contacts, nil // Return empty contacts if file doesn't exist
	}

	// Read contacts file
	data, err := ioutil.ReadFile(contactsFile)
	if err != nil {
		return nil, err
	}

	// Unmarshal contacts
	if len(data) > 0 {
		if err := json.Unmarshal(data, &contacts); err != nil {
			return nil, err
		}
	}

	return contacts, nil
}

// saveContacts saves contacts to a user's contacts file
func saveContacts(username string, contacts ContactsData) error {
	// Ensure directory exists
	if err := os.MkdirAll(config.ContactsDir, 0755); err != nil {
		return err
	}

	// Marshal contacts to JSON
	data, err := json.MarshalIndent(contacts, "", "  ")
	if err != nil {
		return err
	}

	// Write contacts file
	contactsFile := getContactsFile(username)
	return ioutil.WriteFile(contactsFile, data, 0644)
}

// AddContact handles adding a new contact
func AddContact(c *fiber.Ctx) error {
	// Parse request body
	var req AddContactRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request format",
		})
	}

	// Validate required fields
	if req.ContactPublicKey == "" || req.Nickname == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Contact public key and nickname are required",
		})
	}

	// Get username from JWT
	username := middleware.ExtractUsername(c)

	// Load existing contacts
	contacts, err := loadContacts(username)
	if err != nil {
		log.Printf("Error loading contacts: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to load contacts",
		})
	}

	// Add or update contact
	contacts[req.ContactPublicKey] = Contact{
		PublicKey: req.ContactPublicKey,
		Nickname:  req.Nickname,
	}

	// Save contacts
	if err := saveContacts(username, contacts); err != nil {
		log.Printf("Error saving contacts: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to save contact",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Contact added successfully",
	})
}

// GetContacts handles retrieving all contacts for a user
func GetContacts(c *fiber.Ctx) error {
	// Get username from JWT
	username := middleware.ExtractUsername(c)

	// Load contacts
	contacts, err := loadContacts(username)
	if err != nil {
		log.Printf("Error loading contacts: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to load contacts",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":  true,
		"contacts": contacts,
	})
}

// RemoveContact handles removing a contact
func RemoveContact(c *fiber.Ctx) error {
	// Parse request body
	var req RemoveContactRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request format",
		})
	}

	// Validate required fields
	if req.ContactPublicKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Contact public key is required",
		})
	}

	// Get username from JWT
	username := middleware.ExtractUsername(c)

	// Load existing contacts
	contacts, err := loadContacts(username)
	if err != nil {
		log.Printf("Error loading contacts: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to load contacts",
		})
	}

	// Check if contact exists
	if _, exists := contacts[req.ContactPublicKey]; !exists {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Contact not found",
		})
	}

	// Remove contact
	delete(contacts, req.ContactPublicKey)

	// Save contacts
	if err := saveContacts(username, contacts); err != nil {
		log.Printf("Error saving contacts: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to remove contact",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Contact removed successfully",
	})
}
