package routes

import (
	"wave_capacitor/api/handlers"
	"wave_capacitor/middleware"

	"github.com/gofiber/fiber/v2"
)

// SetupRoutes configures all the API routes for the application
func SetupRoutes(app *fiber.App) {
	// Public API endpoints (no authentication required)
	api := app.Group("/api")
	
	// Authentication endpoints
	api.Post("/register", handlers.RegisterUser)
	api.Post("/login", handlers.LoginUser)
	api.Post("/recover_account", handlers.RecoverAccount)

	// Protected API endpoints (require JWT token)
	protected := api.Group("/", middleware.JWTMiddleware)
	
	// User management
	protected.Post("/logout", handlers.LogoutUser)
	protected.Post("/delete_account", handlers.DeleteAccount)
	
	// Key management
	protected.Get("/get_public_key", handlers.GetPublicKey)
	protected.Get("/get_encrypted_private_key", handlers.GetEncryptedPrivateKey)
	
	// Message handling
	protected.Post("/send_message", handlers.SendMessage)
	protected.Get("/get_messages", handlers.GetMessages)
	
	// Contact management
	protected.Post("/add_contact", handlers.AddContact)
	protected.Get("/get_contacts", handlers.GetContacts)
	protected.Post("/remove_contact", handlers.RemoveContact)
	
	// Backup and recovery
	protected.Get("/backup_account", handlers.BackupAccount)
	
	// Health check and status endpoint
	api.Get("/status", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"message": "Wave Capacitor is running",
			"version": "1.0.0",
		})
	})
}
