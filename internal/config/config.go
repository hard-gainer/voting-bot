package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config contains app config
type Config struct {
	MattermostConfig
	TarantoolConfig
}

// Config contains Mattermost config
type MattermostConfig struct {
	MattermostBotHTTPAddr string
	MattermostBotHTTPPort string
	MattermostBotURL      string
	MattermostToken       string
}

// Config contains Tarantool config
type TarantoolConfig struct {
	TarantoolAddr string
	TarantoolUser string
	TarantoolPass string
}

// NewConfig creates a new config
func NewConfig() *Config {
	err := godotenv.Load()
	if err != nil && !os.IsNotExist(err) {
		log.Println("Error loading .env file:", err)
	}

	return &Config{
		MattermostConfig: MattermostConfig{
			MattermostBotHTTPAddr: getEnv("MATTERMOST_BOT_HTTP_ADDR", "http://localhost:8080"),
			MattermostBotHTTPPort: getEnv("MATTERMOST_BOT_HTTP_PORT", ":8080"),
			MattermostBotURL:      getEnv("MATTERMOST_URL", "http://localhost:8065"),
			MattermostToken:       getEnv("MATTERMOST_TOKEN", ""),
		},
		TarantoolConfig: TarantoolConfig{
			TarantoolAddr: getEnv("TARANTOOL_ADDR", "localhost:3301"),
			TarantoolUser: getEnv("TARANTOOL_USER", "storage"),
			TarantoolPass: getEnv("TARANTOOL_PASS", "password"),
		},
	}
}

// getEnv is a helper function for receiving env variables with default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
