package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	// Environment
	Environment string `mapstructure:"ENV"`

	// Server Configuration
	ServerHost string `mapstructure:"SERVER_HOST"`
	ServerPort string `mapstructure:"SERVER_PORT"`

	// Database Configuration
	DBHost     string `mapstructure:"DB_HOST"`
	DBPort     string `mapstructure:"DB_PORT"`
	DBUser     string `mapstructure:"DB_USER"`
	DBPassword string `mapstructure:"DB_PASSWORD"`
	DBName     string `mapstructure:"DB_NAME"`
	DBSSLMode  string `mapstructure:"DB_SSLMODE"`

	// Redis Configuration
	RedisHost string `mapstructure:"REDIS_HOST"`
	RedisPort string `mapstructure:"REDIS_PORT"`
	RedisDB   int    `mapstructure:"REDIS_DB"`

	// LLM Configuration
	LLMDistributedChunkSize int    `mapstructure:"LLM_DISTRIBUTED_CHUNK_SIZE"`
	LLMMaxWorkers          int    `mapstructure:"LLM_MAX_WORKERS"`
	LLMConcurrencyLimit    int    `mapstructure:"LLM_CONCURRENCY_LIMIT"`

	// OpenAI Configuration
	OpenAIAPIKey string `mapstructure:"OPENAI_API_KEY"`
	OpenAIModel  string `mapstructure:"OPENAI_MODEL"`

	// Gemini Configuration
	GeminiAPIKey string `mapstructure:"GEMINI_API_KEY"`
	GeminiModel  string `mapstructure:"GEMINI_MODEL"`

	// Worker Configuration
	WorkerConcurrency   int    `mapstructure:"WORKER_CONCURRENCY"`
	WorkerMaxRetries    int    `mapstructure:"WORKER_MAX_RETRIES"`
	WorkerQueuePriority map[string]int

	// File Processing
	MaxFileSize        int64  `mapstructure:"MAX_FILE_SIZE_MB"`
	TempDir           string `mapstructure:"TEMP_DIR"`
	StreamingChunkSize int    `mapstructure:"STREAMING_CHUNK_SIZE"`
}

// Load loads configuration from environment variables and .env file
func Load() (*Config, error) {
	// Load .env file if exists
	if err := godotenv.Load("../.env"); err != nil {
		// Try parent directory
		if err := godotenv.Load("../../.env"); err != nil {
			log.Println("No .env file found, using environment variables only")
		}
	}

	config := &Config{}

	// Set defaults
	viper.SetDefault("ENV", "development")
	viper.SetDefault("SERVER_HOST", "0.0.0.0")
	viper.SetDefault("SERVER_PORT", "8080")

	// Database defaults
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", "5432")
	viper.SetDefault("DB_NAME", "datagovernance")
	viper.SetDefault("DB_SSLMODE", "disable")

	// Redis defaults
	viper.SetDefault("REDIS_HOST", "localhost")
	viper.SetDefault("REDIS_PORT", "6379")
	viper.SetDefault("REDIS_DB", 0)

	// LLM defaults
	viper.SetDefault("LLM_DISTRIBUTED_CHUNK_SIZE", 50)
	viper.SetDefault("LLM_MAX_WORKERS", 5)
	viper.SetDefault("LLM_CONCURRENCY_LIMIT", 3)
	viper.SetDefault("OPENAI_MODEL", "gpt-4o-mini")
	viper.SetDefault("GEMINI_MODEL", "gemini-1.5-pro")

	// Worker defaults
	viper.SetDefault("WORKER_CONCURRENCY", 10)
	viper.SetDefault("WORKER_MAX_RETRIES", 3)

	// File processing defaults
	viper.SetDefault("MAX_FILE_SIZE_MB", 100)
	viper.SetDefault("TEMP_DIR", "/tmp/uploads")
	viper.SetDefault("STREAMING_CHUNK_SIZE", 1000)

	// Bind environment variables
	viper.AutomaticEnv()

	// Read from env
	config.Environment = viper.GetString("ENV")
	config.ServerHost = viper.GetString("SERVER_HOST")
	config.ServerPort = viper.GetString("SERVER_PORT")

	// Database
	config.DBHost = viper.GetString("DB_HOST")
	config.DBPort = viper.GetString("DB_PORT")
	config.DBUser = viper.GetString("DB_USER")
	config.DBPassword = viper.GetString("DB_PASSWORD")
	config.DBName = viper.GetString("DB_NAME")
	config.DBSSLMode = viper.GetString("DB_SSLMODE")

	// Redis
	config.RedisHost = viper.GetString("REDIS_HOST")
	config.RedisPort = viper.GetString("REDIS_PORT")
	config.RedisDB = viper.GetInt("REDIS_DB")

	// LLM
	config.LLMDistributedChunkSize = viper.GetInt("LLM_DISTRIBUTED_CHUNK_SIZE")
	config.LLMMaxWorkers = viper.GetInt("LLM_MAX_WORKERS")
	config.LLMConcurrencyLimit = viper.GetInt("LLM_CONCURRENCY_LIMIT")

	config.OpenAIAPIKey = viper.GetString("OPENAI_API_KEY")
	config.OpenAIModel = viper.GetString("OPENAI_MODEL")

	config.GeminiAPIKey = viper.GetString("GEMINI_API_KEY")
	config.GeminiModel = viper.GetString("GEMINI_MODEL")

	// Worker
	config.WorkerConcurrency = viper.GetInt("WORKER_CONCURRENCY")
	config.WorkerMaxRetries = viper.GetInt("WORKER_MAX_RETRIES")

	// Set default queue priorities
	config.WorkerQueuePriority = map[string]int{
		"critical":      6,
		"high-priority": 3,
		"default":       1,
	}

	// File processing
	config.MaxFileSize = viper.GetInt64("MAX_FILE_SIZE_MB")
	config.TempDir = viper.GetString("TEMP_DIR")
	config.StreamingChunkSize = viper.GetInt("STREAMING_CHUNK_SIZE")

	// Validate required fields
	if config.DBUser == "" {
		return nil, fmt.Errorf("DB_USER is required")
	}
	if config.DBPassword == "" {
		return nil, fmt.Errorf("DB_PASSWORD is required")
	}
	if config.OpenAIAPIKey == "" && config.GeminiAPIKey == "" {
		return nil, fmt.Errorf("at least one LLM API key is required (OPENAI_API_KEY or GEMINI_API_KEY)")
	}

	return config, nil
}

// GetDatabaseURL constructs the PostgreSQL connection string
func (c *Config) GetDatabaseURL() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode)
}

// GetRedisURL constructs the Redis connection string
func (c *Config) GetRedisURL() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}

// IsProduction returns true if running in production
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// IsDevelopment returns true if running in development
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// LogConfig logs the configuration (hiding sensitive data)
func (c *Config) LogConfig() {
	log.Printf("Configuration loaded:")
	log.Printf("  Environment: %s", c.Environment)
	log.Printf("  Server: %s:%s", c.ServerHost, c.ServerPort)
	log.Printf("  Database: %s:%s/%s", c.DBHost, c.DBPort, c.DBName)
	log.Printf("  Redis: %s:%s (DB: %d)", c.RedisHost, c.RedisPort, c.RedisDB)
	log.Printf("  LLM Chunk Size: %d", c.LLMDistributedChunkSize)
	log.Printf("  LLM Max Workers: %d", c.LLMMaxWorkers)
	log.Printf("  Worker Concurrency: %d", c.WorkerConcurrency)

	// Check API keys without revealing them
	if c.OpenAIAPIKey != "" {
		log.Printf("  OpenAI API Key: [CONFIGURED]")
	} else {
		log.Printf("  OpenAI API Key: [NOT SET]")
	}

	if c.GeminiAPIKey != "" {
		log.Printf("  Gemini API Key: [CONFIGURED]")
	} else {
		log.Printf("  Gemini API Key: [NOT SET]")
	}
}