package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"wattwise/internal/config"
	"wattwise/internal/database"
	"wattwise/internal/handlers"
	"wattwise/internal/mqtt"
	"wattwise/internal/routes"
	"wattwise/internal/services"

	mqttLib "github.com/eclipse/paho.mqtt.golang"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// getWSLIP returns the WSL IP address for display purposes
func getWSLIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "localhost"
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Get addresses for this interface
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// Return first non-loopback IPv4 address
			if ip.To4() != nil {
				return ip.String()
			}
		}
	}

	return "localhost"
}

func main() {
	// ===== SETUP LOGGING =====
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("╔═══════════════════════════════════════╗")
	log.Println("║     🚀 Wattwise Energy Monitor      ║")
	log.Println("║        v1.0 - IoTDB Enabled         ║")
	log.Println("╚═══════════════════════════════════════╝")

	// ===== LOAD CONFIGURATION =====
	log.Println("\n📋 Loading configuration...")
	cfg := config.Load()
	log.Printf("   ✓ Server Port: %s", cfg.Server.Port)
	log.Printf("   ✓ IoTDB: %s:%s", cfg.IoTDB.Host, cfg.IoTDB.Port)
	log.Printf("   ✓ MQTT Broker: %s", cfg.MQTT.Broker)

	// ===== SETUP IOTDB CONNECTION =====
	log.Println("\n🗄️  Initializing IoTDB...")
	db := database.NewIoTDB(cfg.IoTDB)

	// ⭐ PENTING: Jangan panic jika IoTDB error, biarkan jalan dengan dummy mode
	if err := db.Connect(); err != nil {
		log.Printf("⚠️  IoTDB connection failed: %v", err)
		log.Println("   ℹ️  Running in DUMMY MODE - data won't be persisted")
	} else {
		log.Println("✅ IoTDB connected successfully")
		if db.IsEnabled() {
			log.Println("   ✓ Schema initialization completed")
		}
	}

	// ===== SETUP SERVICES =====
	log.Println("\n🔧 Initializing services...")
	energyService := services.NewEnergyService(db)
	log.Println("   ✓ Energy Service initialized")

	// ===== SETUP MQTT CONNECTION =====
	log.Println("\n📡 Initializing MQTT...")
	mqttOpts := mqttLib.NewClientOptions()

	// Get MQTT broker from config
	mqttBroker := cfg.MQTT.Broker
	if mqttBroker == "" {
		mqttBroker = "tcp://127.0.0.1:1883"
		log.Printf("   ⚠️  MQTT_BROKER not set, using default: %s", mqttBroker)
	}

	log.Printf("   ✓ MQTT Broker: %s", mqttBroker)
	mqttOpts.AddBroker(mqttBroker)
	mqttOpts.SetClientID(cfg.MQTT.ClientID)
	mqttOpts.SetCleanSession(true)
	mqttOpts.SetAutoReconnect(true)
	mqttOpts.SetKeepAlive(30 * time.Second)
	mqttOpts.SetConnectTimeout(10 * time.Second)
	mqttOpts.SetMaxReconnectInterval(10 * time.Second)

	// Connection callbacks
	mqttOpts.OnConnect = func(client mqttLib.Client) {
		log.Println("✅ MQTT: Connected to broker")
	}

	mqttOpts.OnConnectionLost = func(client mqttLib.Client, err error) {
		log.Printf("⚠️  MQTT: Connection lost - %v", err)
	}

	mqttOpts.OnReconnecting = func(client mqttLib.Client, opts *mqttLib.ClientOptions) {
		log.Println("🔄 MQTT: Attempting to reconnect...")
	}

	// Create MQTT client
	mqttClient := mqttLib.NewClient(mqttOpts)
	mqttConnected := false

	// Try to connect
	log.Println("   ⏳ Connecting to MQTT broker...")
	token := mqttClient.Connect()
	if token.Wait() && token.Error() == nil {
		log.Println("✅ MQTT connected successfully")
		mqttConnected = true
	} else {
		log.Printf("⚠️  MQTT connection failed: %v", token.Error())
		log.Println("   ℹ️  MQTT will continue to retry in background")
	}

	// ===== SETUP WEBSOCKET HANDLER =====
	log.Println("\n🌐 Initializing WebSocket...")
	wsHandler := handlers.NewWebSocketHandler(db)
	log.Println("   ✓ WebSocket handler initialized")

	// ===== SETUP MQTT SUBSCRIBER =====
	log.Println("\n📥 Initializing MQTT Subscriber...")
	subscriber := mqtt.NewSubscriber(mqttClient, energyService)
	subscriber.SetWebSocketBroadcaster(wsHandler)
	log.Println("   ✓ Subscriber initialized")
	log.Println("   ✓ WebSocket broadcaster connected")

	// Subscribe to energy data jika MQTT connected
	if mqttConnected {
		log.Println("\n🔔 Subscribing to MQTT topics...")
		if err := subscriber.SubscribeToEnergyData(); err != nil {
			log.Printf("❌ Failed to subscribe to topics: %v", err)
			log.Println("   ℹ️  Retrying subscription...")
			// Retry setelah beberapa detik
			go func() {
				time.Sleep(5 * time.Second)
				if err := subscriber.SubscribeToEnergyData(); err != nil {
					log.Printf("❌ Retry failed: %v", err)
				} else {
					log.Println("✅ Subscription successful after retry")
				}
			}()
		} else {
			log.Println("✅ Successfully subscribed to energy topics")
		}
	} else {
		log.Println("⚠️  Skipping MQTT subscription - broker not connected")
		log.Println("   ℹ️  Will attempt to subscribe when connection established")
		// Retry setelah connected
		go func() {
			retries := 0
			for retries < 10 {
				time.Sleep(5 * time.Second)
				if mqttClient.IsConnected() {
					log.Println("🔔 Retrying MQTT subscription after reconnection...")
					if err := subscriber.SubscribeToEnergyData(); err != nil {
						log.Printf("   ❌ Subscription attempt %d failed: %v", retries+1, err)
						retries++
					} else {
						log.Println("   ✅ Subscription successful!")
						break
					}
				}
			}
		}()
	}

	// ===== SETUP FIBER APP =====
	log.Println("\n🔨 Initializing Fiber Framework...")
	app := fiber.New(fiber.Config{
		AppName:       "Wattwise v1.0",
		CaseSensitive: false,
		Immutable:     true,
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	log.Println("   ✓ Middleware configured")

	// ===== SETUP STATIC FILES & ROUTES =====
	log.Println("\n📂 Setting up static files...")
	wd, _ := os.Getwd()
	viewPath := filepath.Join(wd, "..", "view")
	if _, err := os.Stat(viewPath); os.IsNotExist(err) {
		viewPath = filepath.Join(wd, "view")
	}

	// Check if paths exist
	if _, err := os.Stat(viewPath); os.IsNotExist(err) {
		log.Printf("⚠️  View path not found: %s", viewPath)
	} else {
		log.Printf("   ✓ View path: %s", viewPath)
	}

	// Setup routes dengan WebSocket
	routes.SetupWithWebSocket(app, db, wsHandler)
	log.Println("   ✓ API routes configured")

	// Static files
	app.Static("/css", filepath.Join(viewPath, "css"))
	app.Static("/js", filepath.Join(viewPath, "js"))
	app.Static("/view", viewPath)
	log.Println("   ✓ Static files configured")

	// Root redirect
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("/view/login.html")
	})

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":         "ok",
			"service":        "Wattwise Energy Monitor",
			"version":        "1.0.0",
			"iotdb_enabled":  db.IsEnabled(),
			"mqtt_connected": mqttClient.IsConnected(),
			"ws_clients":     wsHandler.GetConnectedClients(),
			"timestamp":      time.Now().Unix(),
		})
	})

	log.Println("   ✓ Health check endpoint available at /health")

	// ===== SETUP GRACEFUL SHUTDOWN =====
	log.Println("\n🛡️  Setting up graceful shutdown...")

	defer func() {
		log.Println("\n🛑 Shutting down gracefully...")

		// Disconnect MQTT
		if mqttClient.IsConnected() {
			log.Println("   ⏳ Disconnecting MQTT...")
			mqttClient.Disconnect(250)
			log.Println("   ✓ MQTT disconnected")
		}

		// Close IoTDB
		log.Println("   ⏳ Closing IoTDB...")
		db.Close()
		log.Println("   ✓ IoTDB closed")

		log.Println("✅ Graceful shutdown completed")
	}()

	// ===== GET WSL IP FOR DISPLAY =====
	wslIP := getWSLIP()

	// ===== START SERVER =====
	log.Println("\n" + "═════════════════════════════════════════════")
	log.Printf("✅ Server starting: http://%s:%s", wslIP, cfg.Server.Port)
	log.Println("═════════════════════════════════════════════")

	// Build clickable URLs
	webUI := fmt.Sprintf("http://%s:%s/view/login.html", wslIP, cfg.Server.Port)
	apiHealth := fmt.Sprintf("http://%s:%s/health", wslIP, cfg.Server.Port)
	wsURL := fmt.Sprintf("ws://%s:%s/ws", wslIP, cfg.Server.Port)
	apiDocs := fmt.Sprintf("http://%s:%s/api/", wslIP, cfg.Server.Port)

	log.Println("\n📝 Available endpoints:")
	log.Printf("   • Web UI:     %s", webUI)
	log.Printf("   • API Health: %s", apiHealth)
	log.Printf("   • WebSocket:  %s", wsURL)
	log.Printf("   • API Docs:   %s", apiDocs)

	log.Println("\n🔐 Default credentials:")
	log.Println("   • Username: admin")
	log.Println("   • Password: admin123")

	log.Println("\n📊 Status:")
	log.Printf("   • IoTDB: %v", db.IsEnabled())
	log.Printf("   • MQTT: %v", mqttClient.IsConnected())

	log.Println("\n🌐 COPY & PASTE THIS URL TO YOUR BROWSER:")
	log.Println("═════════════════════════════════════════════")
	log.Printf("   %s", webUI)
	log.Println("═════════════════════════════════════════════")

	if wslIP != "localhost" {
		log.Println("\n💡 From Windows PowerShell, run this for localhost access:")
		log.Printf("   netsh interface portproxy add v4tov4 listenport=%s listenaddress=0.0.0.0 connectport=%s connectaddress=%s", cfg.Server.Port, cfg.Server.Port, wslIP)
		log.Println("   (Run as Administrator)")
	}

	log.Println("\n⏹️  Press Ctrl+C to stop the server\n")

	// Listen on all interfaces
	listenAddr := "0.0.0.0:" + cfg.Server.Port
	if err := app.Listen(listenAddr); err != nil {
		log.Fatalf("❌ Server error: %v", err)
	}
}
