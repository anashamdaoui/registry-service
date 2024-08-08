package config

import (
	"encoding/json"
	"log"
	"os"
)

// DBConfig holds the database configuration details
type DBConfig struct {
	URI        string `json:"uri"`
	Name       string `json:"name"`
	Collection string `json:"collection"`
}

// Config holds the application configuration
type Config struct {
	LogLevel        string   `json:"log_level"`
	ServerPort      string   `json:"server_port"`
	CheckIntervalMs int      `json:"check_interval_ms"`
	APIKey          string   `json:"api_key"`
	DB              DBConfig `json:"db"`
}

// AppConfig is a global variable that holds the loaded configuration
var AppConfig Config

// LoadConfig loads configuration from a JSON file
func LoadConfig(configFile string) {
	log.Println("", "Loading Static Configuration...")

	file, err := os.Open(configFile)
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&AppConfig); err != nil {
		log.Fatalf("Failed to decode config file: %v", err)
	}

	// Set default values if not specified in the config file
	if AppConfig.CheckIntervalMs == 0 {
		AppConfig.CheckIntervalMs = 100
	}
	log.Println("", "Configuration loaded successfully.")
}
