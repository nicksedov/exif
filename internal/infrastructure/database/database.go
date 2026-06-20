package database

import (
	"fmt"

	"exif-service/internal/domain"
	"exif-service/internal/infrastructure/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Init connects to PostgreSQL and opens the shared database.
// The EXIF service does NOT run AutoMigrate — schema is managed by the main backend.
func Init(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		// PrepareStmt avoids the simple protocol path in the PostgreSQL migrator.
		PrepareStmt: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Verify connectivity to critical tables (read-only check)
	if err := db.AutoMigrate(&domain.ImageMetadata{}, &domain.GeolocationCache{}); err != nil {
		return nil, fmt.Errorf("schema compatibility check failed: %w", err)
	}

	return db, nil
}
