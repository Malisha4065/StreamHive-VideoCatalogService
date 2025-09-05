package db

import (
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/streamhive/video-catalog-api/internal/models"
)

// NewConnection creates a new database connection
func NewConnection() (*gorm.DB, error) {
	dsn := getDSN()

	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	if os.Getenv("DB_LOG_LEVEL") == "debug" {
		config.Logger = logger.Default.LogMode(logger.Info)
	}

	db, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

// RunMigrations runs database migrations
func RunMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Video{},
	&models.Comment{},
	)
}

// getDSN constructs the database connection string from environment variables
func getDSN() string {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "postgres")
	dbname := getEnv("DB_NAME", "video_catalog")
	sslmode := getEnv("DB_SSLMODE", "disable")
	timezone := getEnv("DB_TIMEZONE", "UTC")

	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		host, port, user, password, dbname, sslmode, timezone)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
