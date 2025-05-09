package config

import (
	"encoding/json"
	"os"
	"strconv"
)

type Config struct {
	GRPCPort int    `json:"grpc_port"`
	LogLevel string `json:"log_level"`
}

func LoadConfig(filePath string) (*Config, error) {
	// default config
	cfg := &Config{
		GRPCPort: 50051,
		LogLevel: "info",
	}

	if filePath != "" {
		file, err := os.Open(filePath)
		if err == nil {
			defer file.Close()
			decoder := json.NewDecoder(file)
			if err := decoder.Decode(cfg); err != nil {
				return nil, err
			}
		}
	}

	//	override with env variables(if set)
	if port := os.Getenv("GRPC_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.GRPCPort = p
		}
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		cfg.LogLevel = logLevel
	}

	return cfg, nil
}
