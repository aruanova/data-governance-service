package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestClassification_TableName(t *testing.T) {
	classification := Classification{}
	assert.Equal(t, "classifications", classification.TableName())
}

func TestClassification_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	batch := &Batch{
		OriginalFilename: "test.csv",
		FileHash:         "hash123",
	}
	db.Create(batch)

	classification := &Classification{
		BatchID:      batch.ID,
		RowIndex:     0,
		OriginalData: JSONB{"description": "PROMO TV"},
		CleanedData:  JSONB{"clean_description": "promo tv"},
	}

	assert.Equal(t, uuid.Nil, classification.ID)

	err := db.Create(classification).Error
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, classification.ID)
}

func TestClassification_UniqueBatchRowIndex(t *testing.T) {
	db := setupTestDB(t)

	batch := &Batch{
		OriginalFilename: "test.csv",
		FileHash:         "hash123",
	}
	db.Create(batch)

	// Create first classification
	class1 := &Classification{
		BatchID:      batch.ID,
		RowIndex:     0,
		OriginalData: JSONB{"test": "data1"},
		CleanedData:  JSONB{"clean": "data1"},
	}
	err := db.Create(class1).Error
	assert.NoError(t, err)

	// Try to create another classification with same batch_id and row_index
	class2 := &Classification{
		BatchID:      batch.ID,
		RowIndex:     0, // Same row index - should fail
		OriginalData: JSONB{"test": "data2"},
		CleanedData:  JSONB{"clean": "data2"},
	}
	err = db.Create(class2).Error
	assert.Error(t, err, "should fail due to UNIQUE constraint on (batch_id, row_index)")
}

func TestClassification_IdempotentUpdate(t *testing.T) {
	db := setupTestDB(t)

	batch := &Batch{
		OriginalFilename: "test.csv",
		FileHash:         "hash123",
	}
	db.Create(batch)

	// First classification
	class1 := &Classification{
		BatchID:      batch.ID,
		RowIndex:     0,
		OriginalData: JSONB{"test": "data"},
		CleanedData:  JSONB{"clean": "data"},
		Category:     "Category A",
	}
	db.Create(class1)

	// Simulate worker reprocessing - should update, not create duplicate
	var existing Classification
	result := db.Where("batch_id = ? AND row_index = ?", batch.ID, 0).First(&existing)

	if result.Error == nil {
		// Update existing
		existing.Category = "Category B"
		err := db.Save(&existing).Error
		assert.NoError(t, err)
	}

	// Verify only one record exists
	var count int64
	db.Model(&Classification{}).Where("batch_id = ?", batch.ID).Count(&count)
	assert.Equal(t, int64(1), count)

	// Verify category was updated
	var updated Classification
	db.Where("batch_id = ? AND row_index = ?", batch.ID, 0).First(&updated)
	assert.Equal(t, "Category B", updated.Category)
}

func TestClassification_JSONBFields(t *testing.T) {
	db := setupTestDB(t)

	batch := &Batch{
		OriginalFilename: "test.csv",
		FileHash:         "hash123",
	}
	db.Create(batch)

	originalData := JSONB{
		"LineDescription": "PROMO P1 TV 15 SEG",
		"Amount":          1500.50,
		"Date":            "2024-01-15",
	}

	cleanedData := JSONB{
		"cleanLineDescription": "promo tv seg",
	}

	classification := &Classification{
		BatchID:      batch.ID,
		RowIndex:     0,
		OriginalData: originalData,
		CleanedData:  cleanedData,
	}

	err := db.Create(classification).Error
	assert.NoError(t, err)

	// Load and verify JSONB
	var loaded Classification
	db.First(&loaded, classification.ID)

	assert.Equal(t, "PROMO P1 TV 15 SEG", loaded.OriginalData["LineDescription"])
	assert.Equal(t, 1500.50, loaded.OriginalData["Amount"])
	assert.Equal(t, "promo tv seg", loaded.CleanedData["cleanLineDescription"])
}

func TestClassification_RelationshipWithBatch(t *testing.T) {
	db := setupTestDB(t)

	batch := &Batch{
		OriginalFilename: "test.csv",
		FileHash:         "hash123",
	}
	db.Create(batch)

	classification := &Classification{
		BatchID:      batch.ID,
		RowIndex:     0,
		OriginalData: JSONB{"test": "data"},
		CleanedData:  JSONB{"clean": "data"},
	}
	db.Create(classification)

	// Load classification with batch
	var loaded Classification
	err := db.Preload("Batch").First(&loaded, classification.ID).Error
	assert.NoError(t, err)
	assert.NotNil(t, loaded.Batch)
	assert.Equal(t, "test.csv", loaded.Batch.OriginalFilename)
}