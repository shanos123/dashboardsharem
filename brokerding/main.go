package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"
	"errors"

	_ "modernc.org/sqlite"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type SensorData struct {
	DeviceID    string  `json:"mac"`
	AccelX      float64 `json:"accel_x"`
	AccelY      float64 `json:"accel_y"`
	AccelZ      float64 `json:"accel_z"`
	GyroX       float64 `json:"gyro_x"`
	GyroY       float64 `json:"gyro_y"`
	GyroZ       float64 `json:"gyro_z"`
	Temperature float64 `json:"temperature"`
	Humidity    float64 `json:"humidity"`
	Pressure    float64 `json:"pressure"`
	Timestamp   string  // To store the timestamp as a string
}

var db *sql.DB

// MQTT message handler
var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())

	var data SensorData
	err := json.Unmarshal(msg.Payload(), &data)
	if err != nil {
		fmt.Printf("Error parsing JSON from topic %s: %v\n", msg.Topic(), err)
		return
	}

	seperaterData(data)
	putInSQLiteDB(data)
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("Connected to MQTT broker")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("Connection to MQTT broker lost: %v\n", err)
}

// Helper function to display received data in console
func seperaterData(data SensorData) {
	fmt.Printf("Temperature: %f\n", data.Temperature)
	fmt.Printf("Humidity: %f\n", data.Humidity)
	fmt.Printf("Pressure: %f\n", data.Pressure)
	fmt.Printf("Device ID: %s\n", data.DeviceID)
}

// Store the received sensor data into the SQLite database
func putInSQLiteDB(data SensorData) {
	timestamp := time.Now().Format(time.RFC3339)
	data.Timestamp = timestamp

	// Ensure database file exists
	if _, err := os.Stat("sensor_data.db"); errors.Is(err, os.ErrNotExist) {
		file, err := os.Create("sensor_data.db")
		if err != nil {
			log.Fatal(err)
		}
		file.Close()
		fmt.Println("Database file created")
	} else {
		fmt.Println("Database file exists")
	}

	// Open the database
	db, err := sql.Open("sqlite", "./sensor_data.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create the table if it doesn't exist
	sqlTable := `
	CREATE TABLE IF NOT EXISTS sensor_data (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		deviceid TEXT,
		temp REAL,
		hum REAL,
		press REAL,
		timestamp TEXT
	);`
	_, err = db.Exec(sqlTable)
	if err != nil {
		log.Fatal(err)
	}

	// Insert data
	sqlInsert := `INSERT INTO sensor_data (deviceid, temp, hum, press, timestamp) VALUES (?, ?, ?, ?, ?);`
	_, err = db.Exec(sqlInsert, data.DeviceID, data.Temperature, data.Humidity, data.Pressure, data.Timestamp)
	if err != nil {
		log.Fatal(err)
	}
}

// Fetch sensor data from SQLite database
func fetchSensorData() ([]SensorData, error) {
	rows, err := db.Query("SELECT deviceid, temp, hum, press, timestamp FROM sensor_data ORDER BY timestamp DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var data []SensorData
	for rows.Next() {
		var sd SensorData
		err := rows.Scan(&sd.DeviceID, &sd.Temperature, &sd.Humidity, &sd.Pressure, &sd.Timestamp)
		if err != nil {
			log.Println("Error scanning row:", err)
			continue
		}
		data = append(data, sd)
	}
	return data, nil
}

func renderTemplate(w http.ResponseWriter, r *http.Request) {
    tmpl, err := template.ParseFiles("templates/index.html")
    if err != nil {
        http.Error(w, "Error loading template", http.StatusInternalServerError)
        return
    }

    // Fetch latest sensor data
    data, err := fetchSensorData()
    if err != nil {
        http.Error(w, "Error fetching data", http.StatusInternalServerError)
        return
    }

    // Convert sensor data to JSON string
    jsonData, err := json.Marshal(data)
    if err != nil {
        http.Error(w, "Error encoding data to JSON", http.StatusInternalServerError)
        return
    }

    // Pass the JSON string to the template
    tmpl.Execute(w, string(jsonData))
}

	
	
// Main function
func main() {
	// Initialize the database
	var err error
	db, err = sql.Open("sqlite", "./sensor_data.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Uncomment the following section to enable MQTT functionality

	/*
		var broker = "192.168.2.24"
		var port = 1883
		opts := mqtt.NewClientOptions()
		opts.AddBroker(fmt.Sprintf("tcp://%s:%d", broker, port))
		opts.SetClientID("go_mqtt_client")
		opts.SetUsername("emqx")
		opts.SetPassword("public")
		opts.SetDefaultPublishHandler(messagePubHandler)
		opts.OnConnect = connectHandler
		opts.OnConnectionLost = connectLostHandler

		client := mqtt.NewClient(opts)
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}

		// Subscribe to the MQTT topic
		topic := "hello"
		token := client.Subscribe(topic, 1, nil)
		token.Wait()
		if token.Error() != nil {
			fmt.Printf("Error subscribing to topic %s: %v\n", topic, token.Error())
		} else {
			fmt.Printf("Subscribed to topic: %s\n", topic)
		}
	*/

	// HTTP server setup
	http.HandleFunc("/", renderTemplate)
	fmt.Println("Starting web server on :8080")
	go log.Fatal(http.ListenAndServe(":8080", nil))

	// Keep the program running (especially if MQTT is enabled)
	select {}
}
