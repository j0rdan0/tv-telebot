package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	TVIP           string
	TVMac          string
	TVPort         string
	NgrokURL       string
	NgrokAuthToken string
	ChannelCount   int
}

func LoadConfig() Config {
	channelCount, err := strconv.Atoi(getEnv("CHANNEL_COUNT", "20"))
	if err != nil {
		channelCount = 20
	}

	return Config{
		TVIP:           getEnv("TV_IP", "192.168.0.171"),
		TVMac:          getEnv("TV_MAC", "58:FD:B1:3D:10:3E"),
		TVPort:         getEnv("TV_PORT", "3001"),
		NgrokURL:       getEnv("NGROK_URL", ""),
		NgrokAuthToken: getEnv("NGROK_AUTHTOKEN", ""),
		ChannelCount:   channelCount,
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func UpdateEnv(key, value string) error {
	env, err := godotenv.Read(".env")
	if err != nil {
		env = make(map[string]string)
	}

	env[key] = value
	err = godotenv.Write(env, ".env")
	if err != nil {
		return err
	}

	os.Setenv(key, value)
	return nil
}

func SaveClientKey(newKey string) error {
	return UpdateEnv("client_id", newKey)
}

func SaveNgrokURL(url string) error {
	return UpdateEnv("NGROK_URL", url)
}
