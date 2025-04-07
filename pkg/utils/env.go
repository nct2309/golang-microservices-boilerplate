package utils

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// LoadEnv loads environment variables from a file path (with default is .env)
func LoadEnv(path ...string) error {
	if len(path) == 0 {
		path = []string{".env"}
	}
	return godotenv.Load(path...)
}

// GetEnv retrieves an environment variable or returns a default value
func GetEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// GetEnvAsInt retrieves an environment variable as an integer or returns a default value
func GetEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		var result int
		if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}

// GetEnvDuration gets a duration from environment variable or returns the default value
func GetEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
