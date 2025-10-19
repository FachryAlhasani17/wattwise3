package database

import (
	"fmt"
	"log"
	"time"
	"wattwise/internal/config"
	"wattwise/internal/models"

	"github.com/apache/iotdb-client-go/client"
)

type IoTDB struct {
	session *client.Session
	config 	config.IoTDBConfig
	enabled bool
}

func NewIoTDB(cfg config.IoTDBConfig) *IoTDB {
	return &IoTDB{
		config: 	cfg,
		enabled: false,
	}
}

func (db *IoTDB) Connect() error {
	cfg := &client.Config{
		Host: 	 db.config.Host,
		Port: 	 db.config.Port,
		UserName: db.config.Username,
		Password: db.config.Password,
	}

	session := client.NewSession(cfg)
	if err := session.Open(false, 0); err != nil {
		return err
	}

	db.session = &session
	db.enabled = true
	db.initSchema()
	return nil
}

func (db *IoTDB) Close() {
	if db.enabled && db.session != nil {
		(*db.session).Close()
	}
}

func (db *IoTDB) IsEnabled() bool {
	return db.enabled
}

func (db *IoTDB) initSchema() {
    // 🔧 Ganti DATABASE -> STORAGE GROUP
    _, err := (*db.session).ExecuteStatement("CREATE STORAGE GROUP root.energy")
    if err != nil {
        log.Printf("⚠️ Error creating storage group: %v", err)
    }

    timeseries := []string{
        "CREATE TIMESERIES root.energy.voltage WITH DATATYPE=FLOAT, ENCODING=RLE",
        "CREATE TIMESERIES root.energy.current WITH DATATYPE=FLOAT, ENCODING=RLE",
        "CREATE TIMESERIES root.energy.power WITH DATATYPE=FLOAT, ENCODING=RLE",
        "CREATE TIMESERIES root.energy.energy WITH DATATYPE=FLOAT, ENCODING=RLE",
        "CREATE TIMESERIES root.energy.prediction WITH DATATYPE=FLOAT, ENCODING=RLE",
    }

    for _, ts := range timeseries {
        _, err := (*db.session).ExecuteStatement(ts)
        if err != nil {
            log.Printf("⚠️ Error creating timeseries %s: %v", ts, err)
        }
    }

    log.Println("✅ IoTDB schema initialized!")
}

func (db *IoTDB) GetLatestData(limit int) ([]models.EnergyData, error) {
	if !db.enabled {
		log.Println("⚠️ IoTDB disabled, returning dummy data.")
		return db.getDummyData(limit), nil
	}

	// Query mengambil time dan semua pengukuran (voltage, current, power, energy, prediction)
	query := fmt.Sprintf("SELECT time, voltage, current, power, energy, prediction FROM root.energy ORDER BY time DESC LIMIT %d", limit)

	sessionDataSet, err := (*db.session).ExecuteQueryStatement(query, nil)
	if err != nil {
		return nil, err
	}
	defer sessionDataSet.Close()

	var dataList []models.EnergyData

	for {
		hasNext, err := sessionDataSet.Next()
		if err != nil {
            // Log error saat iterasi data
            log.Printf("Error during dataset iteration: %v", err)
            break 
        }
        if !hasNext {
            break
        }
		
		ts := sessionDataSet.GetTimestamp()

		// ⭐ PERBAIKAN KRUSIAL DI SINI: Menggunakan nama pengukuran sederhana
		data := models.EnergyData{
			Timestamp:  ts, 
			Voltage: 	float64(sessionDataSet.GetFloat("voltage")), // Menggunakan "voltage" saja
			Current: 	float64(sessionDataSet.GetFloat("current")),
			Power: 		float64(sessionDataSet.GetFloat("power")),
			Energy: 	float64(sessionDataSet.GetFloat("energy")),
			Prediction: float64(sessionDataSet.GetFloat("prediction")),
		}

		dataList = append(dataList, data)
	}

	// Setelah perbaikan ini, log Go akan menampilkan error jika ada masalah saat membaca data float.

	return dataList, nil
}

func (db *IoTDB) InsertData(data models.EnergyData) error {
    if !db.enabled {
        log.Println("⚠️ IoTDB not enabled, skipping insert")
        return nil
    }

    timestamp := data.Timestamp
    if timestamp == 0 {
        timestamp = time.Now().UnixMilli()
    }

    measurements := []string{"voltage", "current", "power", "energy", "prediction"}
    values := []interface{}{
        float32(data.Voltage),
        float32(data.Current),
        float32(data.Power),
        float32(data.Energy),
        float32(data.Prediction),
    }
    dataTypes := []client.TSDataType{
        client.FLOAT, client.FLOAT, client.FLOAT, client.FLOAT, client.FLOAT,
    }

    // ✅ Tangkap dua return value (status, err)
    status, err := (*db.session).InsertRecord("root.energy", measurements, dataTypes, values, timestamp)
    if err != nil {
        log.Printf("❌ Failed to insert data to IoTDB: %v", err)
        return err
    }

    // Kalau status != success
    if status != nil && status.GetCode() != 200 {
        log.Printf("⚠️ IoTDB insert returned non-OK status: %v", status)
    } else {
        log.Printf("💾 Inserted to IoTDB: %.1fV %.2fA %.1fW %.3fkWh (t=%d)",
            data.Voltage, data.Current, data.Power, data.Energy, timestamp)
    }

    return nil
}

func (db *IoTDB) getDummyData(limit int) []models.EnergyData {
	var dataList []models.EnergyData
	now := time.Now()

	for i := 0; i < limit; i++ {
		voltage := 220.0 + float64(i%5)*0.5
		current := 5.0 + float64(i%3)*0.2
		power := voltage * current
		energy := 24.0 + float64(i)*0.3
		prediction := energy + 1.5 + float64(i%2)*0.5

		data := models.EnergyData{
			// Mengembalikan UnixMilli int64.
			Timestamp: 	now.Add(-time.Duration(i) * time.Minute).UnixMilli(), 
			Voltage: 	voltage,
			Current: 	current,
			Power: 		power,
			Energy: 	energy,
			Prediction: prediction,
		}
		dataList = append(dataList, data)
	}

	return dataList
}

// Add this method to your IoTDB struct in internal/database/iotdb.go

// GetDataByTimeRange query data dengan time range filter di database level
// Lebih efficient daripada GetLatestData() dengan filtering di application
func (db *IoTDB) GetDataByTimeRange(startTime, endTime int64) ([]models.EnergyData, error) {
	if !db.enabled {
		log.Println("⚠️ IoTDB disabled, returning dummy data.")
		return db.getDummyDataByTimeRange(startTime, endTime), nil
	}

	// Query dengan WHERE clause untuk time range
	query := fmt.Sprintf("SELECT time, voltage, current, power, energy, prediction FROM root.energy WHERE time >= %d AND time <= %d ORDER BY time DESC", startTime, endTime)

	log.Printf("Executing query: %s", query)

	sessionDataSet, err := (*db.session).ExecuteQueryStatement(query, nil)
	if err != nil {
		log.Printf("❌ Error executing query: %v", err)
		return nil, err
	}
	defer sessionDataSet.Close()

	var dataList []models.EnergyData

	for {
		hasNext, err := sessionDataSet.Next()
		if err != nil {
			log.Printf("Error during dataset iteration: %v", err)
			break
		}
		if !hasNext {
			break
		}

		ts := sessionDataSet.GetTimestamp()

		data := models.EnergyData{
			Timestamp:  ts,
			Voltage:    float64(sessionDataSet.GetFloat("voltage")),
			Current:    float64(sessionDataSet.GetFloat("current")),
			Power:      float64(sessionDataSet.GetFloat("power")),
			Energy:     float64(sessionDataSet.GetFloat("energy")),
			Prediction: float64(sessionDataSet.GetFloat("prediction")),
		}

		dataList = append(dataList, data)
	}

	log.Printf("✅ Retrieved %d records from time range %d to %d", len(dataList), startTime, endTime)
	return dataList, nil
}

// getDummyDataByTimeRange generate dummy data untuk time range tertentu
func (db *IoTDB) getDummyDataByTimeRange(startTime, endTime int64) []models.EnergyData {
	var dataList []models.EnergyData

	startTimeObj := time.UnixMilli(startTime)
	endTimeObj := time.UnixMilli(endTime)

	// Generate data setiap 5 menit dalam range
	for ts := startTimeObj; ts.Before(endTimeObj); ts = ts.Add(5 * time.Minute) {
		// Simulate realistic energy data
		hour := ts.Hour()
		
		// Peak hours (08:00 - 18:00): higher consumption
		basePower := 500.0
		if hour >= 8 && hour <= 18 {
			basePower = 1200.0
		}

		voltage := 220.0 + (float64(hour%4) * 0.5)
		current := basePower / voltage
		power := voltage * current
		energy := 0.04 + (float64(hour) * 0.02) // kWh accumulation

		data := models.EnergyData{
			Timestamp:  ts.UnixMilli(),
			Voltage:    voltage,
			Current:    current,
			Power:      power,
			Energy:     energy,
			Prediction: energy + 0.1,
		}
		dataList = append(dataList, data)
	}

	return dataList
}
