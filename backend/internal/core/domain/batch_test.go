package domain

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// setupTestDB creates a PostgreSQL testcontainer for testing
func setupTestDB(t *testing.T) *gorm.DB {
	ctx := context.Background()

	// Create PostgreSQL container
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	// Cleanup container after test
	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate postgres container: %v", err)
		}
	})

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	// Connect to database
	db, err := gorm.Open(pgdriver.Open(connStr), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	// Enable UUID extension
	db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"")

	// Auto migrate all models
	err = db.AutoMigrate(
		&Batch{},
		&Classification{},
		&Prompt{},
		&Validation{},
		&Iteration{},
		&Session{},
		&DedupHash{},
	)
	if err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	return db
}

func TestBatch_TableName(t *testing.T) {
	batch := Batch{}
	assert.Equal(t, "batches", batch.TableName())
}

func TestBatch_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	batch := &Batch{
		OriginalFilename: "test.csv",
		FileHash:         "abc123",
	}

	// Before create, ID should be Nil
	assert.Equal(t, uuid.Nil, batch.ID)

	// Create the batch
	err := db.Create(batch).Error
	assert.NoError(t, err)

	// After create, ID should be set
	assert.NotEqual(t, uuid.Nil, batch.ID)
}

func TestBatch_FileHashUniqueness(t *testing.T) {
	db := setupTestDB(t)

	// Create first batch
	batch1 := &Batch{
		OriginalFilename: "file1.csv",
		FileHash:         "same_hash_123",
	}
	err := db.Create(batch1).Error
	assert.NoError(t, err)

	// Try to create second batch with same hash
	batch2 := &Batch{
		OriginalFilename: "file2.csv",
		FileHash:         "same_hash_123", // Same hash - should fail
	}
	err = db.Create(batch2).Error
	assert.Error(t, err, "should fail due to UNIQUE constraint on file_hash")
}

func TestBatch_StatusValidation(t *testing.T) {
	validStatuses := ValidStatuses()
	expected := []string{
		"uploaded",
		"cleaning",
		"llm_processing",
		"validating",
		"completed",
		"failed",
	}

	assert.Equal(t, expected, validStatuses)
}

func TestBatch_IsValidStatus(t *testing.T) {
	tests := []struct {
		status string
		valid  bool
	}{
		{"uploaded", true},
		{"cleaning", true},
		{"llm_processing", true},
		{"validating", true},
		{"completed", true},
		{"failed", true},
		{"invalid_status", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := IsValidStatus(tt.status)
			assert.Equal(t, tt.valid, result)
		})
	}
}

func TestBatch_DefaultValues(t *testing.T) {
	db := setupTestDB(t)

	batch := &Batch{
		OriginalFilename: "test.csv",
		FileHash:         "hash123",
	}

	err := db.Create(batch).Error
	assert.NoError(t, err)

	// Check defaults
	assert.Equal(t, "uploaded", batch.Status)
	assert.Equal(t, 0, batch.TotalRecords)
	assert.Equal(t, 0, batch.ProcessedRecords)
	assert.NotZero(t, batch.CreatedAt)
	assert.NotZero(t, batch.UpdatedAt)
}

func TestBatch_Relationships(t *testing.T) {
	db := setupTestDB(t)

	// Create batch
	batch := &Batch{
		OriginalFilename: "test.csv",
		FileHash:         "hash123",
	}
	err := db.Create(batch).Error
	assert.NoError(t, err)

	// Create classification for this batch
	classification := &Classification{
		BatchID:      batch.ID,
		RowIndex:     0,
		OriginalData: JSONB{"test": "data"},
		CleanedData:  JSONB{"clean": "data"},
		Category:     "Test Category",
	}
	err = db.Create(classification).Error
	assert.NoError(t, err)

	// Load batch with classifications
	var loadedBatch Batch
	err = db.Preload("Classifications").First(&loadedBatch, batch.ID).Error
	assert.NoError(t, err)
	assert.Len(t, loadedBatch.Classifications, 1)
	assert.Equal(t, "Test Category", loadedBatch.Classifications[0].Category)
}

func TestBatch_UpdatedAtAutoUpdate(t *testing.T) {
	db := setupTestDB(t)

	batch := &Batch{
		OriginalFilename: "test.csv",
		FileHash:         "hash123",
		Status:           "uploaded",
	}
	err := db.Create(batch).Error
	assert.NoError(t, err)

	originalUpdatedAt := batch.UpdatedAt

	// Update batch
	batch.Status = "completed"
	err = db.Save(batch).Error
	assert.NoError(t, err)

	// UpdatedAt should change
	assert.True(t, batch.UpdatedAt.After(originalUpdatedAt))
}