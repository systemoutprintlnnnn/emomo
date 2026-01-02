package repository

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/timmy/emomo/internal/config"
	"github.com/timmy/emomo/internal/domain"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitDB initializes the database connection based on configuration and runs migrations.
// Parameters:
//   - cfg: database configuration including driver and connection settings.
// Returns:
//   - *gorm.DB: initialized database handle.
//   - error: non-nil if connection or migration fails.
func InitDB(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	var db *gorm.DB
	var err error

	log.Printf("[DB] Initializing database with driver: %q", cfg.Driver)

	switch cfg.Driver {
	case "postgres":
		log.Printf("[DB] Using PostgreSQL driver")
		db, err = initPostgres(cfg, gormConfig)
	case "sqlite":
		log.Printf("[DB] Using SQLite driver")
		db, err = initSQLite(cfg, gormConfig)
	default:
		log.Printf("[DB] Unknown driver %q, defaulting to SQLite", cfg.Driver)
		db, err = initSQLite(cfg, gormConfig)
	}

	if err != nil {
		return nil, err
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB instance: %w", err)
	}

	// Set standard connection pool settings
	// These are critical for production stability regardless of the underlying driver
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if cfg.AutoMigrate {
		log.Printf("[DB] AutoMigrate enabled")
		if err := db.AutoMigrate(
			&domain.Meme{},
			&domain.MemeVector{},
			&domain.DataSource{},
			&domain.IngestJob{},
		); err != nil {
			return nil, fmt.Errorf("failed to migrate database: %w", err)
		}
	} else {
		log.Printf("[DB] AutoMigrate disabled")
	}

	return db, nil
}

// initPostgres initializes a PostgreSQL database connection using the unified DSN
func initPostgres(cfg *config.DatabaseConfig, gormConfig *gorm.Config) (*gorm.DB, error) {
	dsn := cfg.DSN()
	// Use postgres.New with PreferSimpleProtocol: true to support Transaction Poolers (like Supabase port 6543)
	// This disables implicit prepared statements which are incompatible with transaction pooling
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}
	return db, nil
}

// initSQLite initializes a SQLite database connection
func initSQLite(cfg *config.DatabaseConfig, gormConfig *gorm.Config) (*gorm.DB, error) {
	// Ensure the directory exists
	if cfg.Path != "" {
		dir := filepath.Dir(cfg.Path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	dsn := cfg.DSN()
	db, err := gorm.Open(sqlite.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SQLite: %w", err)
	}

	// Enable WAL mode for better concurrency (SQLite specific)
	// These are PRAGMA statements, separate from the connection pool settings
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA foreign_keys=ON")

	return db, nil
}
