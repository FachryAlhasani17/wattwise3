package handlers

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"
	"wattwise/internal/database"
	"wattwise/internal/models"
	"wattwise/internal/services"
	"wattwise/internal/utils"

	"github.com/gofiber/fiber/v2"
)

type EnergyHandler struct {
	db            *database.IoTDB
	energyService *services.EnergyService
}

func NewEnergyHandler(db *database.IoTDB) *EnergyHandler {
	return &EnergyHandler{
		db:            db,
		energyService: services.NewEnergyService(db),
	}
}

// GetLatestData gets the most recent energy reading for a device
func (h *EnergyHandler) GetLatestData(c *fiber.Ctx) error {
	deviceID := c.Query("device_id")

	if deviceID == "" {
		dataList, err := h.db.GetLatestData(1)
		if err != nil {
			log.Printf("ERROR: GetLatestData failed: %v", err)
			return utils.ErrorResponse(c, fiber.StatusInternalServerError,
				"Failed to query latest data: "+err.Error())
		}

		if len(dataList) == 0 {
			return utils.SuccessResponse(c, fiber.Map{})
		}

		data := dataList[0]
		response := fiber.Map{
			"timestamp":  data.Timestamp,
			"voltage":    data.Voltage,
			"current":    data.Current,
			"power":      data.Power,
			"energy":     data.Energy,
			"prediction": data.Prediction,
		}

		return utils.SuccessResponse(c, response)
	}

	reading, err := h.energyService.GetLatestData(deviceID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(reading)
}

// GetHistoricalData gets historical energy readings
func (h *EnergyHandler) GetHistoricalData(c *fiber.Ctx) error {
	deviceID := c.Query("device_id")
	if deviceID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "device_id is required",
		})
	}

	limit, _ := strconv.Atoi(c.Query("limit", "100"))
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	var startTime, endTime int64

	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		startTime, _ = strconv.ParseInt(startTimeStr, 10, 64)
	} else {
		startTime = time.Now().Add(-24 * time.Hour).UnixMilli()
	}

	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		endTime, _ = strconv.ParseInt(endTimeStr, 10, 64)
	} else {
		endTime = time.Now().UnixMilli()
	}

	readings, err := h.energyService.GetHistoricalData(deviceID, startTime, endTime, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"device_id": deviceID,
		"count":     len(readings),
		"data":      readings,
	})
}

// ✅ FIXED: GetData returns latest N records with proper limit handling
func (h *EnergyHandler) GetData(c *fiber.Ctx) error {
	limitStr := c.Query("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		log.Printf("⚠️ Invalid limit parameter: %s, using default 50", limitStr)
		limit = 50
	}
	
	log.Printf("📊 GetData called with limit: %d", limit)

	// ✅ Handle special limit values
	if limit == 0 {
		log.Printf("🔍 Request for ALL data detected (limit=0)")
	} else if limit < 0 {
		log.Printf("⚠️ Negative limit (%d), using default 50", limit)
		limit = 50
	} else if limit > 1000000 {
		log.Printf("⚠️ Very large limit (%d), treating as 'fetch all'", limit)
		limit = 0
	}

	if !h.db.IsEnabled() {
		log.Printf("⚠️ IoTDB is not enabled, returning empty array")
		return utils.SuccessResponse(c, []models.EnergyData{})
	}

	log.Printf("📥 Fetching records from IoTDB (limit=%d)...", limit)
	
	dataList, err := h.db.GetLatestData(limit)
	if err != nil {
		log.Printf("❌ ERROR in GetData: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"data":    []models.EnergyData{},
			"error":   err.Error(),
			"message": "Failed to fetch from IoTDB",
		})
	}

	if len(dataList) == 0 {
		log.Printf("⚠️ GetData returned 0 records")
		return utils.SuccessResponse(c, []models.EnergyData{})
	}

	log.Printf("✅ GetData successful: returning %d records", len(dataList))
	return utils.SuccessResponse(c, dataList)
}

// GetFilteredData handles filtered energy data requests
func (h *EnergyHandler) GetFilteredData(c *fiber.Ctx) error {
	deviceID := c.Query("device_id")
	filterType := c.Query("filter", "daily")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	customDays := c.Query("days")

	if deviceID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "device_id is required",
		})
	}

	var results []models.FilteredEnergyData
	var err error

	switch filterType {
	case "hourly":
		if startDate == "" || endDate == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "startDate and endDate are required for hourly filter",
			})
		}
		results, err = h.getHourlyData(deviceID, startDate, endDate)

	case "daily":
		if startDate == "" || endDate == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "startDate and endDate are required for daily filter",
			})
		}
		results, err = h.getDailyData(deviceID, startDate, endDate)

	case "weekly":
		if startDate == "" || endDate == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "startDate and endDate are required for weekly filter",
			})
		}
		results, err = h.getWeeklyData(deviceID, startDate, endDate)

	case "monthly":
		if startDate == "" {
			startDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
		}
		if endDate == "" {
			endDate = time.Now().Format("2006-01-02")
		}
		results, err = h.getMonthlyData(deviceID, startDate, endDate)

	case "custom_days":
		if customDays == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "days parameter required for custom_days filter",
			})
		}
		results, err = h.getCustomDaysData(deviceID, customDays)

	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid filter type. Use: hourly, daily, weekly, monthly, or custom_days",
		})
	}

	if err != nil {
		log.Printf("Error fetching filtered data: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch filtered data: " + err.Error(),
		})
	}

	response := models.FilteredResponse{
		Success: true,
		Filter:  filterType,
		Count:   len(results),
		Data:    results,
	}

	if startDate != "" && endDate != "" {
		response.DateRange = map[string]string{
			"startDate": startDate,
			"endDate":   endDate,
		}
	}

	return c.JSON(response)
}

// getHourlyData aggregates data by hour
func (h *EnergyHandler) getHourlyData(deviceID, startDate, endDate string) ([]models.FilteredEnergyData, error) {
	startTime, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, err
	}
	endTime, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, err
	}
	endTime = endTime.Add(24 * time.Hour)

	startTimestamp := startTime.UnixMilli()
	endTimestamp := endTime.UnixMilli()

	readings, err := h.energyService.GetHistoricalData(deviceID, startTimestamp, endTimestamp, 10000)
	if err != nil {
		return nil, err
	}

	hourMap := make(map[string]*models.FilteredEnergyData)

	for _, reading := range readings {
		timestamp := reading.Timestamp.UnixMilli()
		ts := time.UnixMilli(timestamp)
		hourKey := ts.Format("2006-01-02 15:00:00")

		if _, exists := hourMap[hourKey]; !exists {
			hourMap[hourKey] = &models.FilteredEnergyData{
				TimeGroup: hourKey,
				Hour:      hourKey,
				DataCount: 0,
			}
		}

		data := hourMap[hourKey]
		data.TotalKWh += reading.Energy / 1000
		data.AvgPower += reading.Power
		data.AvgVoltage += reading.Voltage
		data.AvgCurrent += reading.Current

		if reading.Power > data.MaxPower {
			data.MaxPower = reading.Power
		}
		if data.MinPower == 0 || reading.Power < data.MinPower {
			data.MinPower = reading.Power
		}

		data.DataCount++
	}

	var results []models.FilteredEnergyData
	for _, data := range hourMap {
		if data.DataCount > 0 {
			data.AvgPower /= float64(data.DataCount)
			data.AvgVoltage /= float64(data.DataCount)
			data.AvgCurrent /= float64(data.DataCount)
		}
		results = append(results, *data)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Hour > results[j].Hour
	})

	return results, nil
}

// getDailyData aggregates data by day
func (h *EnergyHandler) getDailyData(deviceID, startDate, endDate string) ([]models.FilteredEnergyData, error) {
	startTime, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, err
	}
	endTime, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, err
	}
	endTime = endTime.Add(24 * time.Hour)

	startTimestamp := startTime.UnixMilli()
	endTimestamp := endTime.UnixMilli()

	readings, err := h.energyService.GetHistoricalData(deviceID, startTimestamp, endTimestamp, 10000)
	if err != nil {
		return nil, err
	}

	dayMap := make(map[string]*models.FilteredEnergyData)

	for _, reading := range readings {
		timestamp := reading.Timestamp.UnixMilli()
		ts := time.UnixMilli(timestamp)
		dayKey := ts.Format("2006-01-02")

		if _, exists := dayMap[dayKey]; !exists {
			dayMap[dayKey] = &models.FilteredEnergyData{
				TimeGroup: dayKey,
				Date:      dayKey,
				DataCount: 0,
			}
		}

		data := dayMap[dayKey]
		data.TotalKWh += reading.Energy / 1000
		data.AvgPower += reading.Power
		data.AvgVoltage += reading.Voltage
		data.AvgCurrent += reading.Current

		if reading.Power > data.MaxPower {
			data.MaxPower = reading.Power
		}
		if data.MinPower == 0 || reading.Power < data.MinPower {
			data.MinPower = reading.Power
		}

		data.DataCount++
	}

	var results []models.FilteredEnergyData
	for _, data := range dayMap {
		if data.DataCount > 0 {
			data.AvgPower /= float64(data.DataCount)
			data.AvgVoltage /= float64(data.DataCount)
			data.AvgCurrent /= float64(data.DataCount)
		}
		results = append(results, *data)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Date > results[j].Date
	})

	return results, nil
}

// getWeeklyData aggregates data by week
func (h *EnergyHandler) getWeeklyData(deviceID, startDate, endDate string) ([]models.FilteredEnergyData, error) {
	startTime, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, err
	}
	endTime, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, err
	}
	endTime = endTime.Add(24 * time.Hour)

	startTimestamp := startTime.UnixMilli()
	endTimestamp := endTime.UnixMilli()

	readings, err := h.energyService.GetHistoricalData(deviceID, startTimestamp, endTimestamp, 10000)
	if err != nil {
		return nil, err
	}

	weekMap := make(map[string]*models.FilteredEnergyData)

	for _, reading := range readings {
		timestamp := reading.Timestamp.UnixMilli()
		ts := time.UnixMilli(timestamp)
		year, week := ts.ISOWeek()
		weekKey := fmt.Sprintf("%d-W%02d", year, week)

		weekStart := ts.AddDate(0, 0, -int(ts.Weekday())+1)
		weekStartStr := weekStart.Format("2006-01-02")

		if _, exists := weekMap[weekKey]; !exists {
			weekMap[weekKey] = &models.FilteredEnergyData{
				TimeGroup: weekStartStr,
				Week:      weekKey,
				DataCount: 0,
			}
		}

		data := weekMap[weekKey]
		data.TotalKWh += reading.Energy / 1000
		data.AvgPower += reading.Power
		data.AvgVoltage += reading.Voltage
		data.AvgCurrent += reading.Current

		if reading.Power > data.MaxPower {
			data.MaxPower = reading.Power
		}
		if data.MinPower == 0 || reading.Power < data.MinPower {
			data.MinPower = reading.Power
		}

		data.DataCount++
	}

	var results []models.FilteredEnergyData
	for _, data := range weekMap {
		if data.DataCount > 0 {
			data.AvgPower /= float64(data.DataCount)
			data.AvgVoltage /= float64(data.DataCount)
			data.AvgCurrent /= float64(data.DataCount)
		}
		results = append(results, *data)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Week > results[j].Week
	})

	return results, nil
}

// getMonthlyData aggregates data by month
func (h *EnergyHandler) getMonthlyData(deviceID, startDate, endDate string) ([]models.FilteredEnergyData, error) {
	startTime, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, err
	}
	endTime, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, err
	}
	endTime = endTime.Add(24 * time.Hour)

	startTimestamp := startTime.UnixMilli()
	endTimestamp := endTime.UnixMilli()

	readings, err := h.energyService.GetHistoricalData(deviceID, startTimestamp, endTimestamp, 10000)
	if err != nil {
		return nil, err
	}

	monthMap := make(map[string]*models.FilteredEnergyData)

	for _, reading := range readings {
		timestamp := reading.Timestamp.UnixMilli()
		ts := time.UnixMilli(timestamp)
		monthKey := ts.Format("2006-01")

		if _, exists := monthMap[monthKey]; !exists {
			monthMap[monthKey] = &models.FilteredEnergyData{
				TimeGroup: monthKey + "-01",
				Date:      monthKey + "-01",
				DataCount: 0,
			}
		}

		data := monthMap[monthKey]
		data.TotalKWh += reading.Energy / 1000
		data.AvgPower += reading.Power
		data.AvgVoltage += reading.Voltage
		data.AvgCurrent += reading.Current

		if reading.Power > data.MaxPower {
			data.MaxPower = reading.Power
		}
		if data.MinPower == 0 || reading.Power < data.MinPower {
			data.MinPower = reading.Power
		}

		data.DataCount++
	}

	var results []models.FilteredEnergyData
	for _, data := range monthMap {
		if data.DataCount > 0 {
			data.AvgPower /= float64(data.DataCount)
			data.AvgVoltage /= float64(data.DataCount)
			data.AvgCurrent /= float64(data.DataCount)
		}
		results = append(results, *data)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Date > results[j].Date
	})

	return results, nil
}

// getCustomDaysData gets data for specific selected days
func (h *EnergyHandler) getCustomDaysData(deviceID, daysStr string) ([]models.FilteredEnergyData, error) {
	days := strings.Split(daysStr, ",")
	var allResults []models.FilteredEnergyData

	for _, dayStr := range days {
		dayStr = strings.TrimSpace(dayStr)

		dayTime, err := time.Parse("2006-01-02", dayStr)
		if err != nil {
			continue
		}

		nextDay := dayTime.Add(24 * time.Hour)
		startTimestamp := dayTime.UnixMilli()
		endTimestamp := nextDay.UnixMilli()

		readings, err := h.energyService.GetHistoricalData(deviceID, startTimestamp, endTimestamp, 10000)
		if err != nil {
			continue
		}

		var totalKWh, sumPower, sumVoltage, sumCurrent float64
		var maxPower, minPower float64
		count := 0

		for _, reading := range readings {
			totalKWh += reading.Energy / 1000
			sumPower += reading.Power
			sumVoltage += reading.Voltage
			sumCurrent += reading.Current

			if reading.Power > maxPower {
				maxPower = reading.Power
			}
			if minPower == 0 || reading.Power < minPower {
				minPower = reading.Power
			}

			count++
		}

		if count > 0 {
			result := models.FilteredEnergyData{
				TimeGroup:  dayStr,
				Date:       dayStr,
				TotalKWh:   totalKWh,
				AvgPower:   sumPower / float64(count),
				MaxPower:   maxPower,
				MinPower:   minPower,
				AvgVoltage: sumVoltage / float64(count),
				AvgCurrent: sumCurrent / float64(count),
				DataCount:  count,
			}
			allResults = append(allResults, result)
		}
	}

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Date > allResults[j].Date
	})

	return allResults, nil
}

// GetDailySummary gets daily energy summary
func (h *EnergyHandler) GetDailySummary(c *fiber.Ctx) error {
	deviceID := c.Query("device_id")
	if deviceID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "device_id is required",
		})
	}

	dateStr := c.Query("date")
	var date time.Time
	if dateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "invalid date format, use YYYY-MM-DD",
			})
		}
		date = parsedDate
	} else {
		date = time.Now()
	}

	summary, err := h.energyService.CalculateDailySummary(deviceID, date)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(summary)
}

// GetWeeklySummary gets weekly summary
func (h *EnergyHandler) GetWeeklySummary(c *fiber.Ctx) error {
	deviceID := c.Query("device_id")
	if deviceID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "device_id is required",
		})
	}

	summaries := make([]interface{}, 0, 7)
	now := time.Now()

	for i := 6; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		summary, err := h.energyService.CalculateDailySummary(deviceID, date)
		if err == nil {
			summaries = append(summaries, summary)
		} else {
			summaries = append(summaries, fiber.Map{
				"device_id":    deviceID,
				"date":         date.Format("2006-01-02"),
				"total_energy": 0,
				"avg_power":    0,
				"max_power":    0,
				"min_power":    0,
				"total_cost":   0,
			})
		}
	}

	return c.JSON(fiber.Map{
		"device_id": deviceID,
		"period":    "last_7_days",
		"summaries": summaries,
	})
}

// GetMonthlySummary gets monthly summary
func (h *EnergyHandler) GetMonthlySummary(c *fiber.Ctx) error {
	deviceID := c.Query("device_id")
	if deviceID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "device_id is required",
		})
	}

	monthStr := c.Query("month")
	var targetMonth time.Time
	if monthStr != "" {
		parsedMonth, err := time.Parse("2006-01", monthStr)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "invalid month format, use YYYY-MM",
			})
		}
		targetMonth = parsedMonth
	} else {
		targetMonth = time.Now()
	}

	startOfMonth := time.Date(targetMonth.Year(), targetMonth.Month(), 1, 0, 0, 0, 0, targetMonth.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, -1)

	summaries := make([]interface{}, 0)
	var totalEnergy, totalCost float64

	for d := startOfMonth; d.Before(endOfMonth.AddDate(0, 0, 1)); d = d.AddDate(0, 0, 1) {
		summary, err := h.energyService.CalculateDailySummary(deviceID, d)
		if err == nil {
			summaries = append(summaries, summary)
			totalEnergy += summary.TotalEnergy
			totalCost += summary.TotalCost
		}
	}

	return c.JSON(fiber.Map{
		"device_id":       deviceID,
		"month":           targetMonth.Format("2006-01"),
		"total_energy":    totalEnergy,
		"total_cost":      totalCost,
		"daily_summaries": summaries,
	})
}

// GetRealtimeStats gets real-time statistics
func (h *EnergyHandler) GetRealtimeStats(c *fiber.Ctx) error {
	stats, err := h.energyService.GetRealtimeStats()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(stats)
}

// GetDeviceList gets list of all devices
func (h *EnergyHandler) GetDeviceList(c *fiber.Ctx) error {
	devices, err := h.energyService.GetDeviceList()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"count":   len(devices),
		"devices": devices,
	})
}

// GetDeviceStatus gets status of devices
func (h *EnergyHandler) GetDeviceStatus(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"message": "Device status endpoint",
	})
}

// InsertData inserts energy data
func (h *EnergyHandler) InsertData(c *fiber.Ctx) error {
	var data models.EnergyData
	if err := c.BodyParser(&data); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	deviceID := c.Query("device_id", "ESP32_001")

	if err := h.energyService.SaveEnergyData(deviceID, &data); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Data inserted successfully",
	})
}