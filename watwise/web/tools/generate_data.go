// File: watwise/web/tools/generate_data.go
package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"wattwise/internal/config"
	"wattwise/internal/database"
	"wattwise/internal/models"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	
	fmt.Println("╔════════════════════════════════════════════╗")
	fmt.Println("║  Wattwise Historical Data Generator       ║")
	fmt.Println("╚════════════════════════════════════════════╝")
	fmt.Println()

	// Change to project root
	if err := os.Chdir(".."); err != nil {
		log.Fatalf("❌ Failed to change directory: %v", err)
	}

	wd, _ := os.Getwd()
	log.Printf("📁 Working directory: %s", wd)

	// Load config
	log.Println("\n📋 Loading configuration...")
	cfg := config.Load()
	log.Printf("   ✓ IoTDB: %s:%s", cfg.IoTDB.Host, cfg.IoTDB.Port)

	// Connect to IoTDB
	log.Println("\n🗄️  Connecting to IoTDB...")
	db := database.NewIoTDB(cfg.IoTDB)
	
	if err := db.Connect(); err != nil {
		log.Fatalf("❌ Failed to connect to IoTDB: %v", err)
	}
	defer db.Close()
	
	log.Println("✅ Connected to IoTDB successfully")

	// Get user input
	var days int
	var interval int
	
	fmt.Println("\n📊 Data Generation Parameters:")
	fmt.Print("   How many days of historical data? (1-30): ")
	fmt.Scanln(&days)
	
	if days < 1 || days > 30 {
		days = 7
		log.Printf("⚠️  Invalid input, using default: %d days", days)
	}

	fmt.Print("   Data interval in minutes? (1-60): ")
	fmt.Scanln(&interval)
	
	if interval < 1 || interval > 60 {
		interval = 5
		log.Printf("⚠️  Invalid input, using default: %d minutes", interval)
	}

	// Calculate total records
	recordsPerDay := (24 * 60) / interval
	totalRecords := days * recordsPerDay
	
	fmt.Printf("\n📈 Will generate ~%d records (%d days × %d records/day)\n", 
		totalRecords, days, recordsPerDay)
	fmt.Print("   Continue? (y/n): ")
	
	var confirm string
	fmt.Scanln(&confirm)
	
	if confirm != "y" && confirm != "Y" {
		log.Println("❌ Generation cancelled")
		return
	}

	// Generate data
	log.Println("\n🚀 Starting data generation...")
	
	startTime := time.Now().AddDate(0, 0, -days)
	endTime := time.Now()
	
	successCount := 0
	errorCount := 0
	
	for ts := startTime; ts.Before(endTime); ts = ts.Add(time.Duration(interval) * time.Minute) {
		data := generateRealisticData(ts)
		
		if err := db.InsertData(data); err != nil {
			log.Printf("⚠️  Failed to insert data at %s: %v", ts.Format("2006-01-02 15:04"), err)
			errorCount++
		} else {
			successCount++
			
			// Progress indicator
			if successCount%100 == 0 {
				progress := float64(successCount) / float64(totalRecords) * 100
				log.Printf("⏳ Progress: %d/%d (%.1f%%)", successCount, totalRecords, progress)
			}
		}
	}

	// Summary
	fmt.Println("\n" + "═══════════════════════════════════════════")
	fmt.Println("           GENERATION COMPLETE")
	fmt.Println("═══════════════════════════════════════════")
	fmt.Printf("✅ Successfully inserted: %d records\n", successCount)
	
	if errorCount > 0 {
		fmt.Printf("⚠️  Failed insertions: %d records\n", errorCount)
	}
	
	fmt.Printf("📊 Date range: %s to %s\n", 
		startTime.Format("2006-01-02 15:04"), 
		endTime.Format("2006-01-02 15:04"))
	fmt.Println("═══════════════════════════════════════════")
	
	log.Println("\n✅ Data generation completed!")
}

// generateRealisticData creates realistic energy consumption data
func generateRealisticData(timestamp time.Time) models.EnergyData {
	hour := timestamp.Hour()
	
	// Base power consumption pattern (Watts)
	var basePower float64
	
	// Realistic daily pattern
	switch {
	case hour >= 0 && hour < 6:
		// Night: Low consumption (100-300W)
		basePower = 100 + rand.Float64()*200
	case hour >= 6 && hour < 8:
		// Morning: Medium-high (500-1000W)
		basePower = 500 + rand.Float64()*500
	case hour >= 8 && hour < 17:
		// Daytime: Medium (300-600W)
		basePower = 300 + rand.Float64()*300
	case hour >= 17 && hour < 22:
		// Evening: High (800-1500W)
		basePower = 800 + rand.Float64()*700
	default:
		// Late night: Medium-low (200-500W)
		basePower = 200 + rand.Float64()*300
	}
	
	// Add random variation (±20%)
	variation := 1.0 + (rand.Float64()-0.5)*0.4
	power := basePower * variation
	
	// Calculate realistic voltage (220V ±5%)
	voltage := 220.0 + (rand.Float64()-0.5)*22.0
	
	// Calculate current from power and voltage (I = P/V)
	current := power / voltage
	
	// Frequency (50Hz ±0.5Hz)
	frequency := 50.0 + (rand.Float64()-0.5)*1.0
	
	// Power factor (0.85-0.98)
	powerFactor := 0.85 + rand.Float64()*0.13
	
	// Calculate cumulative energy (kWh)
	// Get a cumulative value based on time elapsed
	hoursSinceStart := timestamp.Sub(time.Now().AddDate(0, 0, -30)).Hours()
	cumulativeEnergy := (basePower * hoursSinceStart) / 1000.0
	
	return models.EnergyData{
		Timestamp:   timestamp.UnixMilli(),
		Voltage:     voltage,
		Current:     current,
		Power:       power,
		Energy:      cumulativeEnergy,
		Frequency:   frequency,
		PowerFactor: powerFactor,
	}
}
