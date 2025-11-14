package models

import "time"

// EnergyData digunakan untuk data yang disimpan di IoTDB
type EnergyData struct {
	Timestamp   int64   `json:"timestamp"` // Unix Millisecond
	Voltage     float64 `json:"voltage"`
	Current     float64 `json:"current"`
	Power       float64 `json:"power"`
	Energy      float64 `json:"energy"`
	Frequency   float64 `json:"frequency"`
	PowerFactor float64 `json:"power_factor"`
	Prediction  float64 `json:"prediction,omitempty"`
}

// EnergyReading untuk response API dengan format time.Time
type EnergyReading struct {
	DeviceID    string    `json:"device_id"`
	Voltage     float64   `json:"voltage"`
	Current     float64   `json:"current"`
	Power       float64   `json:"power"`
	Energy      float64   `json:"energy"`
	Frequency   float64   `json:"frequency"`
	PowerFactor float64   `json:"power_factor"`
	Timestamp   time.Time `json:"timestamp"`
}

// MQTTMessage represents incoming MQTT message from ESP32
// ✅ FIXED: Handle both string dan int64 timestamp
type MQTTMessage struct {
	DeviceID string `json:"device_id"`
	// Timestamp bisa berupa string format "2025-10-20 00:55:31" atau int64
	TimestampStr string  `json:"timestamp,omitempty"` // Jika string
	Timestamp    int64   `json:"timestamp,omitempty"` // Jika int64
	Voltage      float64 `json:"voltage"`
	Current      float64 `json:"current"`
	Power        float64 `json:"power"`
	Energy       float64 `json:"energy"`
	Frequency    float64 `json:"frequency"`
	PowerFactor  float64 `json:"pf"` // ✅ FIXED: Match dengan MQTT payload "pf"
	Rssi         int     `json:"rssi,omitempty"`
	Uptime       int     `json:"uptime,omitempty"`
}

// RealtimeData for WebSocket broadcasting
type RealtimeData struct {
	DeviceID    string  `json:"device_id"`
	DeviceName  string  `json:"device_name"`
	Voltage     float64 `json:"voltage"`
	Current     float64 `json:"current"`
	Power       float64 `json:"power"`
	Energy      float64 `json:"energy"`
	Frequency   float64 `json:"frequency"`
	PowerFactor float64 `json:"power_factor"`
	Status      string  `json:"status"`
	Timestamp   int64   `json:"timestamp"` // Unix millisecond
}

// DeviceStatus untuk tracking device online/offline
type DeviceStatus struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Status     string `json:"status"` // online, offline
	LastSeen   int64  `json:"last_seen"`
}

// DailySummary untuk summary harian
type DailySummary struct {
	DeviceID    string  `json:"device_id"`
	Date        string  `json:"date"`
	TotalEnergy float64 `json:"total_energy"`
	AvgPower    float64 `json:"avg_power"`
	MaxPower    float64 `json:"max_power"`
	MinPower    float64 `json:"min_power"`
	TotalCost   float64 `json:"total_cost"`
}

// AlertData untuk notifikasi
type AlertData struct {
	DeviceID    string  `json:"device_id"`
	AlertType   string  `json:"alert_type"`
	Message     string  `json:"message"`
	Threshold   float64 `json:"threshold"`
	ActualValue float64 `json:"actual_value"`
	Timestamp   int64   `json:"timestamp"`
}

// FilteredEnergyData untuk response data yang sudah diagregasi
type FilteredEnergyData struct {
	TimeGroup  string  `json:"time_group"`  // Bisa berupa date, hour, week, dll
	Date       string  `json:"date"`        // Alias untuk daily view
	Hour       string  `json:"hour"`        // Untuk hourly view
	Week       string  `json:"week"`        // Untuk weekly view
	TotalKWh   float64 `json:"total_kwh"`   // Total energy dalam kWh
	AvgPower   float64 `json:"avg_power"`   // Average power
	MaxPower   float64 `json:"max_power"`   // Maximum power
	MinPower   float64 `json:"min_power"`   // Minimum power
	AvgVoltage float64 `json:"avg_voltage"` // Average voltage
	AvgCurrent float64 `json:"avg_current"` // Average current
	DataCount  int     `json:"data_count"`  // Jumlah data points
}

// FilteredResponse untuk API response
type FilteredResponse struct {
	Success   bool                 `json:"success"`
	Filter    string               `json:"filter"`
	DateRange map[string]string    `json:"date_range,omitempty"`
	Count     int                  `json:"count"`
	Data      []FilteredEnergyData `json:"data"`
}
