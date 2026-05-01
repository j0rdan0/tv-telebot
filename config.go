package main

import (
	"os"
)

type Config struct {
	TVIP     string
	TVMac    string
	TVPort   string
	NgrokURL string
}

func LoadConfig() Config {
	return Config{
		TVIP:     getEnv("TV_IP", "192.168.0.171"),
		TVMac:    getEnv("TV_MAC", "58:FD:B1:3D:10:3E"),
		TVPort:   getEnv("TV_PORT", "3001"),
		NgrokURL: getEnv("NGROK_URL", "https://d4b3-2a04-241e-2306-2980-f1d5-de30-7691-ee9d.ngrok-free.app/bot"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
