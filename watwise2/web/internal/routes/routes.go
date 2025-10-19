package routes

import (
	"wattwise/internal/database"
	"wattwise/internal/handlers"
	"wattwise/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

// Setup - Original function (backward compatible)
func Setup(app *fiber.App, db *database.IoTDB) {
	authHandler := handlers.NewAuthHandler()
	energyHandler := handlers.NewEnergyHandler(db)
	wsHandler := handlers.NewWebSocketHandler(db)

	setupRoutes(app, authHandler, energyHandler, wsHandler)
}

// SetupWithWebSocket - New function dengan integrated WebSocket handler
func SetupWithWebSocket(app *fiber.App, db *database.IoTDB, wsHandler *handlers.WebSocketHandler) {
	authHandler := handlers.NewAuthHandler()
	energyHandler := handlers.NewEnergyHandler(db)

	setupRoutes(app, authHandler, energyHandler, wsHandler)
}

func setupRoutes(app *fiber.App, authHandler *handlers.AuthHandler, energyHandler *handlers.EnergyHandler, wsHandler *handlers.WebSocketHandler) {
	// Auth routes (public)
	api := app.Group("/api")
	auth := api.Group("/auth")
	auth.Post("/login", authHandler.Login)
	auth.Post("/logout", authHandler.Logout)

	// Energy routes (protected)
	energy := api.Group("/energy", middleware.AuthMiddleware())

	// ===== REAL-TIME & LATEST DATA =====
	energy.Get("/latest", energyHandler.GetLatestData)
	energy.Get("/realtime-stats", energyHandler.GetRealtimeStats)

	// ===== HISTORICAL DATA =====
	energy.Get("/history", energyHandler.GetHistoricalData)
	energy.Get("/data", energyHandler.GetData) // Backward compatible

	// ===== NEW: FILTER ENDPOINTS DENGAN SUPPORT BERBAGAI FILTER WAKTU =====
	// Usage: GET /api/energy/filtered?device_id=ESP32_001&filter=daily&startDate=2025-01-15&endDate=2025-01-15
	// Filter types: hourly, daily, weekly, monthly, custom_days
	// Examples:
	//   Daily: /api/energy/filtered?device_id=ESP32_001&filter=daily&startDate=2025-01-15&endDate=2025-01-15
	//   Weekly: /api/energy/filtered?device_id=ESP32_001&filter=weekly&startDate=2025-01-15&endDate=2025-01-21
	//   Monthly: /api/energy/filtered?device_id=ESP32_001&filter=monthly
	//   Custom Days: /api/energy/filtered?device_id=ESP32_001&filter=custom_days&days=2025-01-15,2025-01-16,2025-01-17
	energy.Get("/filtered", energyHandler.GetFilteredData)

	// ===== SUMMARY ENDPOINTS =====
	energy.Get("/summary/daily", energyHandler.GetDailySummary)
	energy.Get("/summary/weekly", energyHandler.GetWeeklySummary)
	energy.Get("/summary/monthly", energyHandler.GetMonthlySummary)

	// ===== INSERT DATA =====
	// Untuk testing atau manual input
	energy.Post("/insert", energyHandler.InsertData)

	// ===== DEVICE MANAGEMENT =====
	devices := api.Group("/devices", middleware.AuthMiddleware())
	devices.Get("/", energyHandler.GetDeviceList)
	devices.Get("/status", energyHandler.GetDeviceStatus)

	// ===== WEBSOCKET =====
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws", websocket.New(wsHandler.HandleConnection))

	// ===== HEALTH CHECK =====
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":     "ok",
			"message":    "Wattwise API is running",
			"version":    "1.0.0",
			"ws_clients": wsHandler.GetConnectedClients(),
		})
	})
}
