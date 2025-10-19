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
	topics := []string{
		"test",              // Direct topic dari ESP32
		"wattwise/energy/+", // Wildcard pattern
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

// ✅ FIXED: handleEnergyMessage with proper timestamp parsing and validation
func (s *Subscriber) handleEnergyMessage(client mqtt.Client, msg mqtt.Message) {
	log.Printf("\n📨 ========== MQTT MESSAGE RECEIVED ==========")
	log.Printf("   Topic: %s", msg.Topic())
	log.Printf("   Payload size: %d bytes", len(msg.Payload()))
	log.Printf("   Raw payload: %s", string(msg.Payload()))

	// ===== PARSE JSON PAYLOAD =====
	var mqttMsg models.MQTTMessage
	if err := json.Unmarshal(msg.Payload(), &mqttMsg); err != nil {
		log.Printf("❌ ERROR: Failed to unmarshal JSON: %v", err)
		log.Printf("   Please check JSON format in ESP32 payload")
		return
	}

	log.Printf("\n📊 ========== PARSED MQTT MESSAGE ==========")

	// Set device ID jika kosong
	if mqttMsg.DeviceID == "" {
		mqttMsg.DeviceID = "ESP32_PZEM"
		log.Printf("⚠️ Device ID was empty, set to: ESP32_PZEM")
	}

	log.Printf("   Device ID: %s", mqttMsg.DeviceID)
	log.Printf("   Voltage: %.2f V", mqttMsg.Voltage)
	log.Printf("   Current: %.3f A", mqttMsg.Current)
	log.Printf("   Power: %.2f W", mqttMsg.Power)
	log.Printf("   Energy: %.4f kWh", mqttMsg.Energy)
	log.Printf("   Frequency: %.1f Hz", mqttMsg.Frequency)
	log.Printf("   Power Factor: %.3f", mqttMsg.PowerFactor)
	log.Printf("   Timestamp (string): %s", mqttMsg.TimestampStr)
	log.Printf("   Timestamp (int64): %d", mqttMsg.Timestamp)

	// ===== VALIDATE DATA =====
	log.Printf("\n✓ ========== VALIDATING DATA ==========")
	if mqttMsg.Voltage <= 0 {
		log.Printf("❌ INVALID: Voltage is %.2f (must be > 0)", mqttMsg.Voltage)
		return
	}
	if mqttMsg.Current < 0 {
		log.Printf("❌ INVALID: Current is %.3f (must be >= 0)", mqttMsg.Current)
		return
	}
	if mqttMsg.Power < 0 {
		log.Printf("❌ INVALID: Power is %.2f (must be >= 0)", mqttMsg.Power)
		return
	}
	log.Printf("✅ Data validation passed")

	// ===== HANDLE TIMESTAMP CONVERSION =====
	log.Printf("\n⏱️ ========== TIMESTAMP CONVERSION ==========")
	var timestampMs int64

	if mqttMsg.TimestampStr != "" {
		// Parse dari string format: "2025-10-20 00:55:31"
		log.Printf("   Parsing timestamp from string: %s", mqttMsg.TimestampStr)
		t, err := time.Parse("2006-01-02 15:04:05", mqttMsg.TimestampStr)
		if err != nil {
			log.Printf("⚠️ Failed to parse timestamp string: %v", err)
			log.Printf("   Using current time instead")
			timestampMs = time.Now().UnixMilli()
		} else {
			timestampMs = t.UnixMilli()
			log.Printf("✅ Parsed timestamp from string: %d ms", timestampMs)
		}
	} else if mqttMsg.Timestamp > 0 {
		// Handle jika timestamp sudah int64
		log.Printf("   Parsing timestamp from int64: %d", mqttMsg.Timestamp)
		if mqttMsg.Timestamp < 1000000000000 {
			// Assume seconds -> convert to ms
			log.Printf("⚠️ Timestamp is in seconds (< 13 digits), converting to ms")
			timestampMs = mqttMsg.Timestamp * 1000
		} else {
			timestampMs = mqttMsg.Timestamp
		}
		log.Printf("✅ Timestamp: %d ms", timestampMs)
	} else {
		// Default: gunakan waktu sekarang
		log.Printf("⚠️ No valid timestamp provided, using current time")
		timestampMs = time.Now().UnixMilli()
		log.Printf("✅ Current timestamp: %d ms", timestampMs)
	}

	// ===== CONVERT TO ENERGYDATA MODEL =====
	log.Printf("\n🔄 ========== CONVERTING TO ENERGYDATA ==========")
	energyData := &models.EnergyData{
		Timestamp:   timestampMs,
		Voltage:     mqttMsg.Voltage,
		Current:     mqttMsg.Current,
		Power:       mqttMsg.Power,
		Energy:      mqttMsg.Energy,
		Frequency:   mqttMsg.Frequency,
		PowerFactor: mqttMsg.PowerFactor,
	}

	log.Printf("✅ Converted EnergyData:")
	log.Printf("   Timestamp: %d ms", energyData.Timestamp)
	log.Printf("   Voltage: %.2f V", energyData.Voltage)
	log.Printf("   Current: %.3f A", energyData.Current)
	log.Printf("   Power: %.2f W", energyData.Power)
	log.Printf("   Energy: %.4f kWh", energyData.Energy)

	// ===== SAVE TO IOTDB =====
	log.Printf("\n💾 ========== SAVING TO IOTDB ==========")
	if err := s.energyService.SaveEnergyData(mqttMsg.DeviceID, energyData); err != nil {
		log.Printf("⚠️ WARNING: Failed to save to IoTDB: %v", err)
		log.Printf("   Continuing to broadcast to WebSocket anyway...")
	} else {
		log.Printf("✅ Successfully saved to IoTDB")
	}

	// ===== UPDATE DEVICE STATUS =====
	log.Printf("\n📡 ========== UPDATING DEVICE STATUS ==========")
	s.updateDeviceStatus(mqttMsg.DeviceID, "online")
	log.Printf("✅ Device status updated to: online")

	// ===== CHECK THRESHOLD ALERTS =====
	log.Printf("\n⚠️ ========== CHECKING THRESHOLD ALERTS ==========")
	if alert := s.energyService.CheckThresholdAlert(mqttMsg.DeviceID, energyData); alert != nil {
		log.Printf("⚠️ ALERT TRIGGERED: %s", alert.AlertType)
		log.Printf("   Message: %s", alert.Message)
		log.Printf("   Threshold: %.2f | Actual: %.2f", alert.Threshold, alert.ActualValue)

		// Broadcast alert ke WebSocket clients
		if s.wsBroadcaster != nil {
			s.wsBroadcaster.BroadcastAlert(*alert)
			log.Printf("✅ Alert broadcasted to WebSocket clients")
		}
	} else {
		log.Printf("✅ All values within acceptable thresholds")
	}

	// ===== PREPARE REALTIME DATA UNTUK WEBSOCKET =====
	log.Printf("\n📤 ========== PREPARING WEBSOCKET BROADCAST ==========")
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

	log.Printf("✅ RealtimeData prepared:")
	log.Printf("   Device: %s", realtimeData.DeviceID)
	log.Printf("   V: %.2f | I: %.3f | P: %.2f | E: %.4f",
		realtimeData.Voltage, realtimeData.Current, realtimeData.Power, realtimeData.Energy)

	// ===== BROADCAST TO WEBSOCKET CLIENTS =====
	log.Printf("\n🔊 ========== BROADCASTING TO WEBSOCKET ==========")
	if s.wsBroadcaster != nil {
		s.wsBroadcaster.BroadcastRealtimeData(realtimeData)
		log.Printf("✅ Data broadcasted to WebSocket clients")
	} else {
		log.Printf("❌ ERROR: WebSocket broadcaster not set!")
	}

	log.Printf("\n✅ ========== MQTT MESSAGE PROCESSING COMPLETE ==========\n")
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
