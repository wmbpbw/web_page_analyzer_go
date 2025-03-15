package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig
	MongoDB  MongoDBConfig
	Analyzer AnalyzerConfig
	Keycloak KeycloakConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

// MongoDBConfig holds MongoDB connection configuration
type MongoDBConfig struct {
	URI            string
	Database       string
	CollectionName string
	Timeout        time.Duration
}

// AnalyzerConfig holds webpage analyzer configuration
type AnalyzerConfig struct {
	RequestTimeout time.Duration
	UserAgent      string
}

// KeycloakConfig holds Keycloak authentication configuration
type KeycloakConfig struct {
	URL          string
	Realm        string
	ClientID     string
	ClientSecret string
}

// New creates a new Config with values from environment variables
func New() (*Config, error) {
	port := getEnv("PORT", "9090")
	readTimeout, err := strconv.Atoi(getEnv("READ_TIMEOUT", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid READ_TIMEOUT: %w", err)
	}

	writeTimeout, err := strconv.Atoi(getEnv("WRITE_TIMEOUT", "10"))
	if err != nil {
		return nil, fmt.Errorf("invalid WRITE_TIMEOUT: %w", err)
	}

	shutdownTimeout, err := strconv.Atoi(getEnv("SHUTDOWN_TIMEOUT", "10"))
	if err != nil {
		return nil, fmt.Errorf("invalid SHUTDOWN_TIMEOUT: %w", err)
	}

	requestTimeout, err := strconv.Atoi(getEnv("REQUEST_TIMEOUT", "10"))
	if err != nil {
		return nil, fmt.Errorf("invalid REQUEST_TIMEOUT: %w", err)
	}

	mongoTimeout, err := strconv.Atoi(getEnv("MONGO_TIMEOUT", "10"))
	if err != nil {
		return nil, fmt.Errorf("invalid MONGO_TIMEOUT: %w", err)
	}

	return &Config{
		Server: ServerConfig{
			Port:            port,
			ReadTimeout:     time.Duration(readTimeout) * time.Second,
			WriteTimeout:    time.Duration(writeTimeout) * time.Second,
			ShutdownTimeout: time.Duration(shutdownTimeout) * time.Second,
		},
		MongoDB: MongoDBConfig{
			URI:            getEnv("MONGO_URI", "mongodb://host.docker.internal:27017"),
			Database:       getEnv("MONGO_DB", "web_analyzer"),
			CollectionName: getEnv("MONGO_COLLECTION", "analyses"),
			Timeout:        time.Duration(mongoTimeout) * time.Second,
		},
		Analyzer: AnalyzerConfig{
			RequestTimeout: time.Duration(requestTimeout) * time.Second,
			UserAgent:      getEnv("USER_AGENT", "WebAnalyzer/1.0"),
		},
		Keycloak: KeycloakConfig{
			URL:          getEnv("KEYCLOAK_URL", "http://host.docker.internal:8080"),
			Realm:        getEnv("KEYCLOAK_REALM", "web-analyzer"),
			ClientID:     getEnv("KEYCLOAK_CLIENT_ID", "web-analyzer-backend"),
			ClientSecret: getEnv("KEYCLOAK_CLIENT_SECRET", ""),
		},
	}, nil
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
