package services

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
	"wattwise/internal/database"
	"wattwise/internal/models"
)

type EnergyService struct {
	db *database.IoTDB
}

func NewEnergyService(db *database.IoTDB) *EnergyService {
	return &EnergyService{
		db: db,
	}
}

// ===== AGGREGATION STRUCTURES =====
type DailyAggregation struct {
	Date      string  `json:"date"`
	TotalKWh  float64 `json:"total_kwh"`
	AvgPower  float64 `json:"avg_power"`
	MaxPower  float64 `json:"max_power"`
	MinPower  float64 `json:"min_power"`
	Count     int     `json:"count"`
}

type HourlyAggregation struct {
	Hour      string  `json:"hour"`
	TotalKWh  float64 `json:"total_kwh"`
	AvgPower  float64 `json:"avg_power"`
	MaxPower  float64 `json:"max_power"`
	MinPower  float64 `json:"min_power"`
	Count     int     `json:"count"`
}

type WeeklyAggregation struct {
	Week      string             `json:"week"`
	StartDate string             `json:"start_date"`
	EndDate   string             `json:"end_date"`
	TotalKWh  float64            `json:"total_kwh"`
	AvgDaily  float64            `json:"avg_daily_kwh"`
	Daily     []DailyAggregation `json:"daily_breakdown"`
}

type MonthlyAggregation struct {
	Month    string             `json:"month"`
	TotalKWh float64            `json:"total_kwh"`
	AvgDaily float64            `json:"avg_daily_kwh"`
	Daily    []DailyAggregation `json:"daily_breakdown"`
}

// ===== ORIGINAL FUNCTIONS (MAINTAINED) =====

// SaveEnergyData menyimpan data energi ke IoTDB
func (s *EnergyService) SaveEnergyData(deviceID string, data *models.EnergyData) error {
	log.Printf("✅ Saving energy data for device: %s (Power: %.2fW)", deviceID, data.Power)
	
	// TODO: Implement actual save to IoTDB
	// Contoh: return s.db.InsertData(*data)
	
	return nil
}

// GetLatestData mendapatkan data terbaru dari device
func (s *EnergyService) GetLatestData(deviceID string) (*models.EnergyReading, error) {
	log.Printf("Getting latest data for device: %s", deviceID)
	
	// TODO: Query dari IoTDB
	// Sementara return dummy data
	return &models.EnergyReading{
		DeviceID:    deviceID,
		Voltage:     220.0,
		Current:     1.5,
		Power:       330.0,
		Energy:      1.2,
		Frequency:   50.0,
		PowerFactor: 0.95,
		Timestamp:   time.Now(),
	}, nil
}

// GetHistoricalData mendapatkan data historis dengan range waktu
func (s *EnergyService) GetHistoricalData(deviceID string, startTime, endTime int64, limit int) ([]models.EnergyReading, error) {
	log.Printf("Getting historical data for device: %s", deviceID)
	return []models.EnergyReading{}, nil
}

// CalculateDailySummary menghitung summary harian
func (s *EnergyService) CalculateDailySummary(deviceID string, date time.Time) (*models.DailySummary, error) {
	return &models.DailySummary{
		DeviceID:    deviceID,
		Date:        date.Format("2006-01-02"),
		TotalEnergy: 10.5,
		AvgPower:    500.0,
		MaxPower:    1200.0,
		MinPower:    100.0,
		TotalCost:   15162.0,
	}, nil
}

// CheckThresholdAlert cek apakah data melebihi threshold
func (s *EnergyService) CheckThresholdAlert(deviceID string, data *models.EnergyData) *models.AlertData {
	const (
		maxPower   = 2200.0
		maxCurrent = 10.0
		minVoltage = 200.0
		maxVoltage = 240.0
	)
	
	if data.Power > maxPower {
		return &models.AlertData{
			DeviceID:    deviceID,
			AlertType:   "high_power",
			Message:     fmt.Sprintf("Power exceeded: %.2fW", data.Power),
			Threshold:   maxPower,
			ActualValue: data.Power,
			Timestamp:   data.Timestamp,
		}
	}
	
	if data.Current > maxCurrent {
		return &models.AlertData{
			DeviceID:    deviceID,
			AlertType:   "high_current",
			Message:     fmt.Sprintf("Current exceeded: %.2fA", data.Current),
			Threshold:   maxCurrent,
			ActualValue: data.Current,
			Timestamp:   data.Timestamp,
		}
	}
	
	if data.Voltage < minVoltage || data.Voltage > maxVoltage {
		return &models.AlertData{
			DeviceID:    deviceID,
			AlertType:   "voltage_abnormal",
			Message:     fmt.Sprintf("Voltage abnormal: %.2fV", data.Voltage),
			Threshold:   minVoltage,
			ActualValue: data.Voltage,
			Timestamp:   data.Timestamp,
		}
	}
	
	return nil
}

// GetDeviceList mendapatkan daftar device yang terdaftar
func (s *EnergyService) GetDeviceList() ([]string, error) {
	return []string{"ESP32_001"}, nil
}

// GetRealtimeStats mendapatkan statistik real-time semua device
func (s *EnergyService) GetRealtimeStats() (map[string]interface{}, error) {
	return map[string]interface{}{
		"total_devices":  1,
		"online_devices": 1,
		"total_power":    330.0,
		"total_energy":   1.2,
		"estimated_cost": 1732.8,
	}, nil
}

// ===== NEW FILTER FUNCTIONS =====

// ConvertTimestamp convert timestamp ke time.Time (handle int64 atau time.Time)
func convertTimestamp(ts interface{}) time.Time {
	switch v := ts.(type) {
	case int64:
		// Assume milliseconds dari IoTDB
		return time.UnixMilli(v)
	case time.Time:
		return v
	case string:
		// Try parse string
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t
		}
		if t, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
			return t
		}
		return time.Now()
	default:
		return time.Now()
	}
}

// GetDataByDateRange query data berdasarkan date range
func (s *EnergyService) GetDataByDateRange(deviceID string, startDate, endDate time.Time) ([]models.EnergyData, error) {
	startTime := startDate.UnixMilli()
	endTime := endDate.UnixMilli()

	log.Printf("Querying data for device %s from %s to %s", deviceID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	// Query menggunakan method baru GetDataByTimeRange
	readings, err := s.db.GetDataByTimeRange(startTime, endTime)
	if err != nil {
		log.Printf("Error querying data by date range: %v", err)
		return nil, err
	}

	return readings, nil
}

// GetDataBySpecificDays query data untuk specific days
// Format: "2025-01-15,2025-01-16,2025-01-17"
func (s *EnergyService) GetDataBySpecificDays(deviceID string, daysParam string) ([]models.EnergyData, error) {
	days := strings.Split(daysParam, ",")
	var allReadings []models.EnergyData

	// Get all data
	readings, err := s.db.GetLatestData(10000)
	if err != nil {
		log.Printf("Error querying data: %v", err)
		return nil, err
	}

	// Parse target dates
	var targetDates []time.Time
	for _, dayStr := range days {
		dayStr = strings.TrimSpace(dayStr)
		date, err := time.Parse("2006-01-02", dayStr)
		if err != nil {
			log.Printf("Invalid date format: %s", dayStr)
			continue
		}
		targetDates = append(targetDates, date)
	}

	// Filter readings by specific dates
	for _, reading := range readings {
		ts := convertTimestamp(reading.Timestamp)
		readingDate := ts.Format("2006-01-02")

		for _, targetDate := range targetDates {
			if readingDate == targetDate.Format("2006-01-02") {
				allReadings = append(allReadings, reading)
				break
			}
		}
	}

	log.Printf("Retrieved %d readings for %d specific days", len(allReadings), len(days))
	return allReadings, nil
}

// AggregateDailyData aggregate hourly/raw data ke daily
func (s *EnergyService) AggregateDailyData(readings []models.EnergyData) []DailyAggregation {
	dailyMap := make(map[string][]models.EnergyData)

	// Group by date
	for _, reading := range readings {
		ts := convertTimestamp(reading.Timestamp)
		date := ts.Format("2006-01-02")
		dailyMap[date] = append(dailyMap[date], reading)
	}

	// Sort dates
	var dates []string
	for date := range dailyMap {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	// Calculate aggregation
	var result []DailyAggregation
	for _, date := range dates {
		dayReadings := dailyMap[date]
		agg := s.calculateDailyStats(dayReadings, date)
		result = append(result, agg)
	}

	return result
}

// AggregateHourlyData aggregate readings by hour
func (s *EnergyService) AggregateHourlyData(readings []models.EnergyData) []HourlyAggregation {
	hourlyMap := make(map[string][]models.EnergyData)

	// Group by hour
	for _, reading := range readings {
		ts := convertTimestamp(reading.Timestamp)
		hour := ts.Format("2006-01-02 15:00")
		hourlyMap[hour] = append(hourlyMap[hour], reading)
	}

	// Sort hours
	var hours []string
	for hour := range hourlyMap {
		hours = append(hours, hour)
	}
	sort.Strings(hours)

	// Calculate aggregation
	var result []HourlyAggregation
	for _, hour := range hours {
		hourReadings := hourlyMap[hour]
		agg := s.calculateHourlyStats(hourReadings, hour)
		result = append(result, agg)
	}

	return result
}

// AggregateWeeklyData aggregate to weekly with daily breakdown
func (s *EnergyService) AggregateWeeklyData(readings []models.EnergyData) []WeeklyAggregation {
	// First aggregate daily
	daily := s.AggregateDailyData(readings)

	// Group daily into weeks
	weeklyMap := make(map[string][]DailyAggregation)
	var weeks []string

	for _, d := range daily {
		date, _ := time.Parse("2006-01-02", d.Date)
		year, week := date.ISOWeek()
		weekKey := fmt.Sprintf("%d-W%02d", year, week)

		if _, exists := weeklyMap[weekKey]; !exists {
			weeks = append(weeks, weekKey)
		}
		weeklyMap[weekKey] = append(weeklyMap[weekKey], d)
	}

	sort.Strings(weeks)

	// Calculate weekly aggregation
	var result []WeeklyAggregation
	for _, week := range weeks {
		dailyList := weeklyMap[week]
		totalKwh := float64(0)

		for _, d := range dailyList {
			totalKwh += d.TotalKWh
		}

		startDate := dailyList[0].Date
		endDate := dailyList[len(dailyList)-1].Date

		agg := WeeklyAggregation{
			Week:      week,
			StartDate: startDate,
			EndDate:   endDate,
			TotalKWh:  totalKwh,
			AvgDaily:  totalKwh / float64(len(dailyList)),
			Daily:     dailyList,
		}
		result = append(result, agg)
	}

	return result
}

// AggregateMonthlyData aggregate to monthly with daily breakdown
func (s *EnergyService) AggregateMonthlyData(readings []models.EnergyData) MonthlyAggregation {
	// Get daily data
	daily := s.AggregateDailyData(readings)

	// Calculate monthly total
	totalKwh := float64(0)
	for _, d := range daily {
		totalKwh += d.TotalKWh
	}

	var month string
	if len(daily) > 0 {
		date, _ := time.Parse("2006-01-02", daily[0].Date)
		month = date.Format("2006-01")
	}

	avgDaily := float64(0)
	if len(daily) > 0 {
		avgDaily = totalKwh / float64(len(daily))
	}

	return MonthlyAggregation{
		Month:    month,
		TotalKWh: totalKwh,
		Daily:    daily,
		AvgDaily: avgDaily,
	}
}

// ===== HELPER FUNCTIONS =====

func (s *EnergyService) calculateDailyStats(readings []models.EnergyData, date string) DailyAggregation {
	if len(readings) == 0 {
		return DailyAggregation{Date: date}
	}

	totalKwh := float64(0)
	totalPower := float64(0)
	maxPower := readings[0].Power
	minPower := readings[0].Power

	for _, r := range readings {
		totalKwh += r.Energy
		totalPower += r.Power
		if r.Power > maxPower {
			maxPower = r.Power
		}
		if r.Power < minPower {
			minPower = r.Power
		}
	}

	return DailyAggregation{
		Date:     date,
		TotalKWh: totalKwh,
		AvgPower: totalPower / float64(len(readings)),
		MaxPower: maxPower,
		MinPower: minPower,
		Count:    len(readings),
	}
}

func (s *EnergyService) calculateHourlyStats(readings []models.EnergyData, hour string) HourlyAggregation {
	if len(readings) == 0 {
		return HourlyAggregation{Hour: hour}
	}

	totalKwh := float64(0)
	totalPower := float64(0)
	maxPower := readings[0].Power
	minPower := readings[0].Power

	for _, r := range readings {
		totalKwh += r.Energy
		totalPower += r.Power
		if r.Power > maxPower {
			maxPower = r.Power
		}
		if r.Power < minPower {
			minPower = r.Power
		}
	}

	return HourlyAggregation{
		Hour:     hour,
		TotalKWh: totalKwh,
		AvgPower: totalPower / float64(len(readings)),
		MaxPower: maxPower,
		MinPower: minPower,
		Count:    len(readings),
	}
}