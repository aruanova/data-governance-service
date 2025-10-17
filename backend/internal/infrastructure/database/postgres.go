package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/alejandroruanova/data-governance-service/backend/internal/pkg/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PostgresDB wraps the GORM database connection
type PostgresDB struct {
	DB     *gorm.DB
	logger *slog.Logger
}

// NewPostgresDB creates a new PostgreSQL connection using GORM
func NewPostgresDB(cfg *config.DatabaseConfig, appLogger *slog.Logger) (*PostgresDB, error) {
	// Build connection string (DSN)
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Database,
		cfg.SSLMode,
	)

	// Configure GORM logger
	gormLogger := logger.Default.LogMode(logger.Silent)
	if cfg.LogLevel == "debug" {
		gormLogger = logger.Default.LogMode(logger.Info)
	}

	// Open connection with GORM
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                 gormLogger,
		SkipDefaultTransaction: true, // Better performance
		PrepareStmt:            true, // Prepared statements cache
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying sql.DB to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxOpenConns(cfg.MaxConnections)
	sqlDB.SetMaxIdleConns(cfg.MinConnections)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.MaxConnLifetime) * time.Minute)
	sqlDB.SetConnMaxIdleTime(time.Duration(cfg.MaxConnIdleTime) * time.Minute)

	// Ping to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	appLogger.Info("database connection established",
		slog.String("host", cfg.Host),
		slog.Int("port", cfg.Port),
		slog.String("database", cfg.Database),
	)

	return &PostgresDB{
		DB:     db,
		logger: appLogger,
	}, nil
}

// Close closes the database connection
func (db *PostgresDB) Close() error {
	db.logger.Info("closing database connection")
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Ping checks if the database is reachable
func (db *PostgresDB) Ping(ctx context.Context) error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// Health returns health status of the database
func (db *PostgresDB) Health(ctx context.Context) map[string]interface{} {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return map[string]interface{}{
			"status": "down",
			"error":  err.Error(),
		}
	}

	stats := sqlDB.Stats()

	return map[string]interface{}{
		"status":              "up",
		"max_open_conns":      stats.MaxOpenConnections,
		"open_connections":    stats.OpenConnections,
		"in_use":              stats.InUse,
		"idle":                stats.Idle,
		"wait_count":          stats.WaitCount,
		"wait_duration":       stats.WaitDuration.String(),
		"max_idle_closed":     stats.MaxIdleClosed,
		"max_lifetime_closed": stats.MaxLifetimeClosed,
	}
}

// AutoMigrate runs automatic migrations for the given models
func (db *PostgresDB) AutoMigrate(models ...interface{}) error {
	db.logger.Info("running auto migrations")
	if err := db.DB.AutoMigrate(models...); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	db.logger.Info("migrations completed successfully")
	return nil
}