package handlers

import (
	"log"
	"wave_capacitor/middleware"
	"wave_capacitor/models"

	"github.com/gofiber/fiber/v2"
)

// GetPublicKey returns the public key of the authenticated user
func GetPublicKey(c *fiber.Ctx) error {
	// Get username from JWT
	username := middleware.ExtractUsername(c)

	// Get user from database
	user, err := models.GetUser(username)
	if err != nil {
		log.Printf("Error retrieving user for public key: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to retrieve user information",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":    true,
		"public_key": user.PublicKey,
	})
}

// GetEncryptedPrivateKey returns the encrypted private key of the authenticated user
func GetEncryptedPrivateKey(c *fiber.Ctx) error {
	// Get username from JWT
	username := middleware.ExtractUsername(c)

	// Get user from database
	user, err := models.GetUser(username)
	if err != nil {
		log.Printf("Error retrieving user for encrypted private key: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to retrieve user information",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":              true,
		"encrypted_private_key": user.EncryptedPrivKey,
	})
}
