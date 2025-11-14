package database

import (
	"fmt"
	"log"
	"sort"
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
    log.Println("🔧 Initializing IoTDB schema...")
    
    storageGroupCmd := "CREATE STORAGE GROUP root.wattwise"
    log.Printf("   Executing: %s", storageGroupCmd)
    _, err := (*db.session).ExecuteStatement(storageGroupCmd)
    if err != nil {
        log.Printf("⚠️ Error creating storage group: %v", err)
    }

    timeseries := []string{
        "CREATE TIMESERIES root.wattwise.voltage WITH DATATYPE=DOUBLE, ENCODING=GORILLA, COMPRESSOR=LZ4",
        "CREATE TIMESERIES root.wattwise.current WITH DATATYPE=DOUBLE, ENCODING=GORILLA, COMPRESSOR=LZ4",
        "CREATE TIMESERIES root.wattwise.power WITH DATATYPE=DOUBLE, ENCODING=GORILLA, COMPRESSOR=LZ4",
        "CREATE TIMESERIES root.wattwise.energy WITH DATATYPE=DOUBLE, ENCODING=GORILLA, COMPRESSOR=LZ4",
        "CREATE TIMESERIES root.wattwise.frequency WITH DATATYPE=DOUBLE, ENCODING=GORILLA, COMPRESSOR=LZ4",
        "CREATE TIMESERIES root.wattwise.power_factor WITH DATATYPE=DOUBLE, ENCODING=GORILLA, COMPRESSOR=LZ4",
        "CREATE TIMESERIES root.wattwise.prediction WITH DATATYPE=FLOAT, ENCODING=RLE, COMPRESSOR=SNAPPY",
    }

    for _, ts := range timeseries {
        log.Printf("   Executing: %s", ts)
        _, err := (*db.session).ExecuteStatement(ts)
        if err != nil {
            log.Printf("⚠️ Info creating timeseries: %v (mungkin sudah ada)", err)
        }
    }

    log.Println("✅ IoTDB schema initialized!")
}

func (db *IoTDB) GetLatestData(limit int) ([]models.EnergyData, error) {
	if !db.enabled {
		log.Println("⚠️ IoTDB disabled, returning dummy data.")
		return db.getDummyData(limit), nil
	}

	// ✅ FIX: Query WITHOUT ORDER BY (IoTDB 1.3.2 bug dengan ORDER BY)
	// Kita sort di Go level saja
	query := fmt.Sprintf("SELECT voltage, current, power, energy, frequency, power_factor FROM root.wattwise LIMIT %d", limit)
	sessionDataSet, err := (*db.session).ExecuteQueryStatement(query, nil)
	if err != nil {
        log.Printf("⚠️ Query error: %v", err)
        log.Printf("   Query was: %s", query)
        return nil, err
	}
	defer sessionDataSet.Close()

	var dataList []models.EnergyData

	for {
		hasNext, err := sessionDataSet.Next()
		if err != nil {
            log.Printf("❌ Error during dataset iteration: %v", err)
            break 
        }
        if !hasNext {
            break
        }
		
		ts := sessionDataSet.GetTimestamp()

		data := models.EnergyData{
			Timestamp:   ts,
			Voltage:    	float64(sessionDataSet.GetDouble("voltage")),
			Current:    	float64(sessionDataSet.GetDouble("current")),
			Power:      	float64(sessionDataSet.GetDouble("power")),
			Energy:     	float64(sessionDataSet.GetDouble("energy")),
			Frequency:     	float64(sessionDataSet.GetDouble("frequency")),
			PowerFactor:   	float64(sessionDataSet.GetDouble("power_factor")),
		}

		dataList = append(dataList, data)
	}

	// ✅ SORT DESC by timestamp di Go level
	sort.Slice(dataList, func(i, j int) bool {
		return dataList[i].Timestamp > dataList[j].Timestamp
	})

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

    measurements := []string{"voltage", "current", "power", "energy", "frequency", "power_factor"}
    values := []interface{}{
        float64(data.Voltage),
        float64(data.Current),
        float64(data.Power),
        float64(data.Energy),
        float64(data.Frequency),
        float64(data.PowerFactor),
    }
    dataTypes := []client.TSDataType{
        client.DOUBLE, client.DOUBLE, client.DOUBLE, client.DOUBLE, client.DOUBLE, client.DOUBLE,
    }

    status, err := (*db.session).InsertRecord("root.wattwise", measurements, dataTypes, values, timestamp)
    
    if err != nil {
        errMsg := err.Error()
        
        if contains(errMsg, "doesn't exist") || contains(errMsg, "session") || contains(errMsg, "statement") {
            log.Printf("⚠️ IoTDB session error detected, attempting reconnect...")
            
            if db.session != nil {
                (*db.session).Close()
            }
            
            if reconnectErr := db.Connect(); reconnectErr != nil {
                log.Printf("❌ Failed to reconnect to IoTDB: %v", reconnectErr)
                return fmt.Errorf("IoTDB reconnect failed: %w", reconnectErr)
            }
            
            log.Println("✅ IoTDB reconnected successfully, retrying insert...")
            
            status, err = (*db.session).InsertRecord("root.wattwise", measurements, dataTypes, values, timestamp)
            if err != nil {
                log.Printf("❌ Retry insert also failed: %v", err)
                return err
            }
        } else {
            log.Printf("❌ Failed to insert data to IoTDB: %v", err)
            return err
        }
    }

    if status != nil && status.GetCode() != 200 {
        log.Printf("⚠️ IoTDB insert returned non-OK status: %v", status)
    } else {
        log.Printf("✅ Inserted to IoTDB: V=%.2fV I=%.3fA P=%.2fW E=%.5fkWh T=%d",
            data.Voltage, data.Current, data.Power, data.Energy, timestamp)
    }

    return nil
}

func contains(s, substr string) bool {
    return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
    for i := 0; i <= len(s)-len(substr); i++ {
        if s[i:i+len(substr)] == substr {
            return true
        }
    }
    return false
}

func (db *IoTDB) getDummyData(limit int) []models.EnergyData {
	var dataList []models.EnergyData
	now := time.Now()

	for i := 0; i < limit; i++ {
		voltage := 220.0 + float64(i%5)*0.5
		current := 5.0 + float64(i%3)*0.2
		power := voltage * current
		energy := 24.0 + float64(i)*0.3

		data := models.EnergyData{
			Timestamp: 	now.Add(-time.Duration(i) * time.Minute).UnixMilli(), 
			Voltage: 	voltage,
			Current: 	current,
			Power: 		power,
			Energy: 	energy,
			Frequency:	50.0,
			PowerFactor: 0.95,
		}
		dataList = append(dataList, data)
	}

	return dataList
}

func (db *IoTDB) GetDataByTimeRange(startTime, endTime int64) ([]models.EnergyData, error) {
	if !db.enabled {
		log.Println("⚠️ IoTDB disabled, returning dummy data.")
		return db.getDummyDataByTimeRange(startTime, endTime), nil
	}

	// ✅ FIX: Query WITHOUT ORDER BY
	query := fmt.Sprintf("SELECT voltage, current, power, energy, frequency, power_factor FROM root.wattwise WHERE time >= %d AND time <= %d", startTime, endTime)	log.Printf("Executing query: %s", query)

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
			Timestamp:   ts,
			Voltage:    	float64(sessionDataSet.GetDouble("voltage")),
			Current:    	float64(sessionDataSet.GetDouble("current")),
			Power:      	float64(sessionDataSet.GetDouble("power")),
			Energy:     	float64(sessionDataSet.GetDouble("energy")),
			Frequency:     	float64(sessionDataSet.GetDouble("frequency")),
			PowerFactor:   	float64(sessionDataSet.GetDouble("power_factor")),
		}

		dataList = append(dataList, data)
	}

	// ✅ SORT DESC by timestamp di Go level
	sort.Slice(dataList, func(i, j int) bool {
		return dataList[i].Timestamp > dataList[j].Timestamp
	})

	log.Printf("✅ Retrieved %d records from time range %d to %d", len(dataList), startTime, endTime)
	return dataList, nil
}

func (db *IoTDB) getDummyDataByTimeRange(startTime, endTime int64) []models.EnergyData {
	var dataList []models.EnergyData

	startTimeObj := time.UnixMilli(startTime)
	endTimeObj := time.UnixMilli(endTime)

	for ts := startTimeObj; ts.Before(endTimeObj); ts = ts.Add(5 * time.Minute) {
		hour := ts.Hour()
		
		basePower := 500.0
		if hour >= 8 && hour <= 18 {
			basePower = 1200.0
		}

		voltage := 220.0 + (float64(hour%4) * 0.5)
		current := basePower / voltage
		power := voltage * current
		energy := 0.04 + (float64(hour) * 0.02)

		data := models.EnergyData{
			Timestamp:  ts.UnixMilli(),
			Voltage:    voltage,
			Current:    current,
			Power:      power,
			Energy:     energy,
			Frequency:	50.0,
			PowerFactor: 0.95,
		}
		dataList = append(dataList, data)
	}

	return dataList
}