package handlers

import (
	"log"
	"wattwise/internal/utils"

	"github.com/gofiber/fiber/v2"
)

type AuthHandler struct {
	users map[string]string
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
	User    *User  `json:"user,omitempty"`
}

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{
		users: map[string]string{
			"admin": "admin123",
		},
	}
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest

	if err := c.BodyParser(&req); err != nil {
		log.Printf("❌ Failed to parse request body: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(LoginResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	log.Printf("🔐 Login attempt: %s", req.Username)

	// Validate credentials
	password, exists := h.users[req.Username]
	if !exists || password != req.Password {
		log.Printf("❌ Login failed: %s", req.Username)
		return c.Status(fiber.StatusUnauthorized).JSON(LoginResponse{
			Success: false,
			Message: "Username atau password salah",
		})
	}

	// Generate JWT token - FIX: Handle error!
	token, err := utils.GenerateToken(req.Username)
	if err != nil {
		log.Printf("❌ Failed to generate token: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(LoginResponse{
			Success: false,
			Message: "Gagal membuat token autentikasi",
		})
	}

	user := &User{
		ID:       1,
		Username: req.Username,
		Email:    req.Username + "@wattwise.com",
	}

	log.Printf("✅ Login successful: %s (token: %s...)", req.Username, token[:20])

	return c.Status(fiber.StatusOK).JSON(LoginResponse{
		Success: true,
		Message: "Login berhasil",
		User:    user,
		Token:   token,
	})
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Logout berhasil",
	})
}
