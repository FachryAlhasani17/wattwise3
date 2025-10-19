package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
	"wattwise/internal/models"
	"wattwise/internal/services"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type WebSocketBroadcaster interface {
	BroadcastRealtimeData(data models.RealtimeData)
	BroadcastAlert(alert models.AlertData)
}

type Subscriber struct {
	client        mqtt.Client
	energyService *services.EnergyService
	wsBroadcaster WebSocketBroadcaster
	deviceStatus  map[string]*models.DeviceStatus
	statusMutex   sync.RWMutex
}

func NewSubscriber(client mqtt.Client, energyService *services.EnergyService) *Subscriber {
	return &Subscriber{
		client:        client,
		energyService: energyService,
		deviceStatus:  make(map[string]*models.DeviceStatus),
	}
}

// SetWebSocketBroadcaster sets the WebSocket handler untuk broadcasting
func (s *Subscriber) SetWebSocketBroadcaster(broadcaster WebSocketBroadcaster) {
	s.wsBroadcaster = broadcaster
	log.Println("✅ WebSocket broadcaster connected to MQTT subscriber")
}

// SubscribeToEnergyData subscribes to energy data from ESP32 devices
func (s *Subscriber) SubscribeToEnergyData() error {
	if !s.client.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	// Subscribe ke topic "test" (sesuai dengan ESP32 publish)
	// Atau ganti dengan "wattwise/energy/+" jika prefer wildcard
	topics := []string{
		"test",                  // Direct topic dari ESP32
		"wattwise/energy/+",    // Wildcard pattern
	}

	for _, topic := range topics {
		log.Printf("🔔 Attempting to subscribe to topic: %s", topic)
		
		token := s.client.Subscribe(topic, 1, s.handleEnergyMessage)
		if token.Wait() && token.Error() != nil {
			log.Printf("⚠️ Failed to subscribe to %s: %v", topic, token.Error())
			continue
		}
		
		log.Printf("✅ Successfully subscribed to: %s", topic)
	}

	// Start device status checker
	go s.checkDeviceStatus()

	return nil
}

// handleEnergyMessage processes incoming MQTT messages from ESP32
func (s *Subscriber) handleEnergyMessage(client mqtt.Client, msg mqtt.Message) {
	log.Printf("📨 Received MQTT message on topic: %s (payload: %d bytes)", msg.Topic(), len(msg.Payload()))
	
	var mqttMsg models.MQTTMessage
	if err := json.Unmarshal(msg.Payload(), &mqttMsg); err != nil {
		log.Printf("❌ Error unmarshaling MQTT message: %v", err)
		log.Printf("   Raw payload: %s", string(msg.Payload()))
		return
	}

	// Set device ID jika kosong
	if mqttMsg.DeviceID == "" {
		mqttMsg.DeviceID = "ESP32_PZEM"
	}

	log.Printf("📊 Parsed MQTT message: Device=%s, V=%.1f, I=%.2f, P=%.1f, E=%.3f", 
		mqttMsg.DeviceID, mqttMsg.Voltage, mqttMsg.Current, mqttMsg.Power, mqttMsg.Energy)

	// Convert timestamp dari milliseconds (dari ESP32) 
	// ESP32 send timestamp dalam ms, tapi ada kalanya dalam seconds
	timestampMs := mqttMsg.Timestamp
	if mqttMsg.Timestamp < 1000000000000 {
		// Jika < 13 digit, assume seconds -> convert to ms
		timestampMs = mqttMsg.Timestamp * 1000
	}

	// Convert to EnergyData model untuk IoTDB
	energyData := &models.EnergyData{
		Timestamp:   timestampMs,
		Voltage:     mqttMsg.Voltage,
		Current:     mqttMsg.Current,
		Power:       mqttMsg.Power,
		Energy:      mqttMsg.Energy,
		Frequency:   mqttMsg.Frequency,
		PowerFactor: mqttMsg.PowerFactor,
	}

	// Save to IoTDB (if enabled)
	if err := s.energyService.SaveEnergyData(mqttMsg.DeviceID, energyData); err != nil {
		log.Printf("⚠️ Warning saving to IoTDB: %v (will continue broadcasting)", err)
	}

	// Update device status
	s.updateDeviceStatus(mqttMsg.DeviceID, "online")

	// Check for alerts
	if alert := s.energyService.CheckThresholdAlert(mqttMsg.DeviceID, energyData); alert != nil {
		log.Printf("⚠️ ALERT: %s - %s", alert.AlertType, alert.Message)

		// Broadcast alert ke WebSocket clients
		if s.wsBroadcaster != nil {
			s.wsBroadcaster.BroadcastAlert(*alert)
		}
	}

	// Prepare realtime data untuk WebSocket broadcast
	realtimeData := models.RealtimeData{
		DeviceID:    mqttMsg.DeviceID,
		DeviceName:  mqttMsg.DeviceID,
		Voltage:     mqttMsg.Voltage,
		Current:     mqttMsg.Current,
		Power:       mqttMsg.Power,
		Energy:      mqttMsg.Energy,
		Frequency:   mqttMsg.Frequency,
		PowerFactor: mqttMsg.PowerFactor,
		Status:      "online",
		Timestamp:   timestampMs,
	}

	// Broadcast to WebSocket clients
	if s.wsBroadcaster != nil {
		s.wsBroadcaster.BroadcastRealtimeData(realtimeData)
	} else {
		log.Printf("⚠️ WebSocket broadcaster not set, data won't be sent to frontend")
	}

	log.Printf("✅ Successfully processed: %s | %.1fV | %.2fA | %.1fW | %.3fkWh",
		mqttMsg.DeviceID, mqttMsg.Voltage, mqttMsg.Current, mqttMsg.Power, mqttMsg.Energy)
}

// handleStatusMessage processes device status messages
func (s *Subscriber) handleStatusMessage(client mqtt.Client, msg mqtt.Message) {
	log.Printf("📊 Status message: %s - %s", msg.Topic(), string(msg.Payload()))

	var statusMsg map[string]interface{}
	if err := json.Unmarshal(msg.Payload(), &statusMsg); err != nil {
		log.Printf("❌ Error unmarshaling status message: %v", err)
		return
	}

	if deviceID, ok := statusMsg["device_id"].(string); ok {
		if status, ok := statusMsg["status"].(string); ok {
			s.updateDeviceStatus(deviceID, status)
		}
	}
}

// updateDeviceStatus updates device status in memory
func (s *Subscriber) updateDeviceStatus(deviceID, status string) {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()

	s.deviceStatus[deviceID] = &models.DeviceStatus{
		DeviceID:   deviceID,
		DeviceName: deviceID,
		Status:     status,
		LastSeen:   time.Now().UnixMilli(),
	}
	
	log.Printf("📊 Device status updated: %s -> %s", deviceID, status)
}

// checkDeviceStatus checks if devices are still online
func (s *Subscriber) checkDeviceStatus() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.statusMutex.Lock()
		now := time.Now().UnixMilli()

		for deviceID, status := range s.deviceStatus {
			// Jika tidak ada data dalam 60 detik, tandai offline
			if now-status.LastSeen > 60000 && status.Status == "online" {
				status.Status = "offline"
				log.Printf("⚠️ Device %s is now OFFLINE (no data for 60s)", deviceID)
			}
		}
		s.statusMutex.Unlock()
	}
}

// GetDeviceStatus returns current status of a device
func (s *Subscriber) GetDeviceStatus(deviceID string) *models.DeviceStatus {
	s.statusMutex.RLock()
	defer s.statusMutex.RUnlock()

	return s.deviceStatus[deviceID]
}

// GetAllDeviceStatus returns status of all devices
func (s *Subscriber) GetAllDeviceStatus() []*models.DeviceStatus {
	s.statusMutex.RLock()
	defer s.statusMutex.RUnlock()

	statuses := make([]*models.DeviceStatus, 0, len(s.deviceStatus))
	for _, status := range s.deviceStatus {
		statuses = append(statuses, status)
	}

	return statuses
}