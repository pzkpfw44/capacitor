package handlers

import (
	"encoding/base64"
	"fmt"
	"log"
	"wave_capacitor/middleware"
	"wave_capacitor/models"
	"wave_capacitor/utils"

	"github.com/gofiber/fiber/v2"
)

// RegisterRequest defines the structure for registration requests
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginRequest defines the structure for login requests
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RegisterUser handles user registration, generating a Kyber512 keypair
func RegisterUser(c *fiber.Ctx) error {
	// Parse request body
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request format",
		})
	}

	// Validate inputs
	if req.Username == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Username and password are required",
		})
	}

	// Check if user already exists
	exists, err := models.UserExists(req.Username)
	if err != nil {
		log.Printf("Error checking if user exists: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Database error",
		})
	}
	if exists {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Username already exists",
		})
	}

	// Generate Kyber512 key pair
	pubKey, privKey, err := utils.GenerateKyber512Keys()
	if err != nil {
		log.Printf("Error generating key pair: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to generate cryptographic keys",
		})
	}

	// Encrypt the private key
	// In a real implementation, we would use the user's password here
	encryptedPrivKey, err := utils.EncryptPrivateKey(privKey)
	if err != nil {
		log.Printf("Error encrypting private key: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to secure private key",
		})
	}

	// Store user in database
	err = models.CreateUser(req.Username, pubKey, []byte(encryptedPrivKey))
	if err != nil {
		log.Printf("Error creating user: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to create user account",
		})
	}

	// Generate JWT token
	token, err := middleware.GenerateToken(req.Username)
	if err != nil {
		log.Printf("Error generating token: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to generate authentication token",
		})
	}

	// Return success with token and public key
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success":    true,
		"message":    "User registered successfully",
		"token":      token,
		"public_key": base64.StdEncoding.EncodeToString(pubKey),
	})
}

// LoginUser authenticates a user and returns their JWT token
func LoginUser(c *fiber.Ctx) error {
	// Parse request body
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request format",
		})
	}

	// Validate inputs
	if req.Username == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Username and password are required",
		})
	}

	// Check if user exists
	user, err := models.GetUser(req.Username)
	if err != nil {
		log.Printf("Login failed - user not found: %s", req.Username)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid username or password",
		})
	}

	// In a real implementation, we would verify the password here

	// Generate JWT token
	token, err := middleware.GenerateToken(req.Username)
	if err != nil {
		log.Printf("Error generating token: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to generate authentication token",
		})
	}

	// Return success with token
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Welcome back, %s", req.Username),
		"token":   token,
		"user": fiber.Map{
			"username":   user.Username,
			"public_key": user.PublicKey,
		},
	})
}

// LogoutUser handles user logout (mostly a placeholder as JWT is stateless)
func LogoutUser(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Logged out successfully",
	})
}

// DeleteAccount removes a user account and all associated data
func DeleteAccount(c *fiber.Ctx) error {
	// Get username from JWT
	username := middleware.ExtractUsername(c)

	// Delete user from database
	err := models.DeleteUser(username)
	if err != nil {
		log.Printf("Error deleting user %s: %v", username, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to delete account",
		})
	}

	// In a real implementation, we would also delete messages, contacts, etc.

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Account deleted successfully",
	})
}

// NOTE: RecoverAccount function was moved to backup_handler.go
// to avoid function name conflicts
