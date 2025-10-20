package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Server ServerConfig
	IoTDB  IoTDBConfig
	MQTT   MQTTConfig
	JWT    JWTConfig
}

type ServerConfig struct {
	Port string
	Env  string
}

type IoTDBConfig struct {
	Host     string
	Port     string
	Username string
	Password string
}

type MQTTConfig struct {
	Broker   string
	Port     string
	ClientID string
	Username string
	Password string
}

type JWTConfig struct {
	Secret     string
	ExpireTime int
}

func Load() *Config {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  No .env file found, using environment variables")
	}

	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
			Env:  getEnv("ENV", "development"),
		},
		IoTDB: IoTDBConfig{
			Host:     getEnv("IOTDB_HOST", "46.8.226.46"),
			Port:     getEnv("IOTDB_PORT", "6667"),
			Username: getEnv("IOTDB_USERNAME", "root"),
			Password: getEnv("IOTDB_PASSWORD", "root"),
		},
		MQTT: MQTTConfig{
			// ✅ FIX: Ubah dari "tcp://192.168.1.100 :1883" ke "tcp://192.168.1.100:1883" (TANPA SPASI)
			// Juga bisa ganti IP ke 127.0.0.1 jika MQTT di localhost
			Broker:   getEnv("MQTT_BROKER", "tcp://46.8.226.46:1883"),
			Port:     getEnv("MQTT_PORT", "1883"),
			ClientID: getEnv("MQTT_CLIENT_ID", "wattwise_server"),
			Username: getEnv("MQTT_USERNAME", "iotproject2025"),
			Password: getEnv("MQTT_PASSWORD", "abc54321"),
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", "wattwise-secret-key-change-in-production"),
			ExpireTime: 24, // hours
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}