package main

import (
	"log"
	"os"
	"path/filepath"

	"wattwise/internal/config"
	"wattwise/internal/database"
	"wattwise/internal/handlers"
	"wattwise/internal/mqtt"
	"wattwise/internal/routes"
	"wattwise/internal/services"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	mqttLib "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	log.Println("🚀 Starting Wattwise Energy Monitor...")

	cfg := config.Load()
	db := database.NewIoTDB(cfg.IoTDB)

	if err := db.Connect(); err != nil {
		log.Println("⚠️  IoTDB DUMMY MODE")
	} else {
		log.Println("✅ IoTDB connected")
	}

	energyService := services.NewEnergyService(db)

	// MQTT Setup
	mqttOpts := mqttLib.NewClientOptions()
	mqttBroker := cfg.MQTT.Broker
	if mqttBroker == "" {
		mqttBroker = "tcp://192.168.1.100:1883"
	}

	log.Printf("🔌 MQTT Broker: %s", mqttBroker)  // ← TAMBAH LOG INI
	mqttOpts.AddBroker(mqttBroker)

	mqttOpts.AddBroker(mqttBroker)
	mqttOpts.SetClientID(cfg.MQTT.ClientID)
	mqttOpts.SetAutoReconnect(true)

	mqttClient := mqttLib.NewClient(mqttOpts)
	mqttConnected := false

	if token := mqttClient.Connect(); token.Wait() && token.Error() == nil {
		log.Println("✅ MQTT connected")
		mqttConnected = true
		defer mqttClient.Disconnect(250)
	} else {
		log.Println("⚠️  MQTT disconnected")
	}

	wsHandler := handlers.NewWebSocketHandler(db)
	subscriber := mqtt.NewSubscriber(mqttClient, energyService)
	subscriber.SetWebSocketBroadcaster(wsHandler)

	if mqttConnected {
    if err := subscriber.SubscribeToEnergyData(); err != nil {
        log.Printf("❌ Failed to subscribe: %v", err)
    } else {
        log.Println("✅ MQTT subscription successful")
    }
	} else {
    log.Println("⚠️ MQTT not connected, skipping subscription")
	}

	app := fiber.New(fiber.Config{
		AppName: "Wattwise v1.0",
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New())

	wd, _ := os.Getwd()
	viewPath := filepath.Join(wd, "..", "view")
	if _, err := os.Stat(viewPath); os.IsNotExist(err) {
		viewPath = filepath.Join(wd, "view")
	}

	routes.SetupWithWebSocket(app, db, wsHandler)

	app.Static("/css", filepath.Join(viewPath, "css"))
	app.Static("/js", filepath.Join(viewPath, "js"))
	app.Static("/view", viewPath)

	app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("/view/login.html")
	})

	log.Println("✅ Server: http://localhost:" + cfg.Server.Port)
	log.Fatal(app.Listen(":" + cfg.Server.Port))
}
