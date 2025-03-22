package middleware

import (
	"time"
	"wave_capacitor/config"

	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// JWTMiddleware protects specific routes requiring authentication
var JWTMiddleware = jwtware.New(jwtware.Config{
	SigningKey: jwtware.SigningKey{Key: config.GetJWTSecret()},
	ErrorHandler: func(c *fiber.Ctx, err error) error {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   "Unauthorized",
			"message": "Invalid or expired token",
		})
	},
})

// GenerateToken creates a new JWT token for a user
func GenerateToken(username string) (string, error) {
	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(), // Token expires in 24 hours
		"iat":      time.Now().Unix(),                     // Issued at time
	})

	// Generate encoded token
	return token.SignedString(config.GetJWTSecret())
}

// ExtractUsername gets the username from the JWT token
func ExtractUsername(c *fiber.Ctx) string {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	return claims["username"].(string)
}
