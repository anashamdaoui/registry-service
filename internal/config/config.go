package config

import (
	"encoding/json"
	"io"
	"log"
	"os"
)

type Config struct {
	LogLevel string `json:"log_level"`
	Port     string `json:"server_port"`
	APIKey   string `json:"api_key"`
}

var AppConfig Config

func LoadConfig(configFile string) {
	file, err := os.Open(configFile)
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	if err = json.Unmarshal(bytes, &AppConfig); err != nil {
		log.Fatalf("Failed to unmarshal config file: %v", err)
	}
}
