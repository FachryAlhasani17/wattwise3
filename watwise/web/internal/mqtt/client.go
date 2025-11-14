package mqtt

import (
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Client struct {
	client   mqtt.Client
	broker   string
	clientID string
}

// NewClient creates a new MQTT client
func NewClient(broker, clientID string) *Client {
	return &Client{
		broker:   broker,
		clientID: clientID,
	}
}

// Connect establishes connection to MQTT broker
func (c *Client) Connect() error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(c.broker)
	opts.SetClientID(c.clientID)
	opts.SetUsername("") // Add username if needed
	opts.SetPassword("") // Add password if needed
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)
	opts.SetCleanSession(true)

	// Connection callbacks
	opts.OnConnect = func(client mqtt.Client) {
		log.Println("✅ Connected to MQTT broker")
	}

	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		log.Printf("⚠️  Connection lost to MQTT broker: %v", err)
	}

	opts.OnReconnecting = func(client mqtt.Client, opts *mqtt.ClientOptions) {
		log.Println("🔄 Reconnecting to MQTT broker...")
	}

	c.client = mqtt.NewClient(opts)

	// Connect to broker
	token := c.client.Connect()
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %v", token.Error())
	}

	return nil
}

// Disconnect closes connection to MQTT broker
func (c *Client) Disconnect() {
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(250)
		log.Println("✅ Disconnected from MQTT broker")
	}
}

// IsConnected checks if client is connected
func (c *Client) IsConnected() bool {
	return c.client != nil && c.client.IsConnected()
}

// GetClient returns the underlying MQTT client
func (c *Client) GetClient() mqtt.Client {
	return c.client
}

// Publish publishes a message to a topic
func (c *Client) Publish(topic string, payload interface{}) error {
	if !c.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	token := c.client.Publish(topic, 1, false, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish message: %v", token.Error())
	}

	return nil
}
