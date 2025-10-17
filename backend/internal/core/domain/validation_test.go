package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidation_TableName(t *testing.T) {
	validation := Validation{}
	assert.Equal(t, "validations", validation.TableName())
}

func TestValidation_IdempotencyKey(t *testing.T) {
	db := setupTestDB(t)

	// Setup batch and classification
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

	// First validation with idempotency key
	validation1 := &Validation{
		BatchID:          batch.ID,
		ClassificationID: classification.ID,
		UserFeedback:     "correct",
		IdempotencyKey:   "key123",
	}
	err := db.Create(validation1).Error
	assert.NoError(t, err)

	// Try to create another validation with same idempotency key
	validation2 := &Validation{
		BatchID:          batch.ID,
		ClassificationID: classification.ID,
		UserFeedback:     "incorrect",
		IdempotencyKey:   "key123", // Same key - should fail
	}
	err = db.Create(validation2).Error
	assert.Error(t, err, "should fail due to UNIQUE constraint on idempotency_key")
}

func TestValidation_UniqueClassificationValidation(t *testing.T) {
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

	// First validation
	validation1 := &Validation{
		BatchID:          batch.ID,
		ClassificationID: classification.ID,
		UserFeedback:     "correct",
	}
	err := db.Create(validation1).Error
	assert.NoError(t, err)

	// Try to validate same classification again
	validation2 := &Validation{
		BatchID:          batch.ID,
		ClassificationID: classification.ID, // Same classification
		UserFeedback:     "incorrect",
	}
	err = db.Create(validation2).Error
	assert.Error(t, err, "should fail - one validation per classification")
}

func TestValidation_ValidFeedbacks(t *testing.T) {
	expected := []string{"correct", "incorrect", "uncertain"}
	assert.Equal(t, expected, ValidFeedbacks())
}

func TestValidation_IsValidFeedback(t *testing.T) {
	tests := []struct {
		feedback string
		valid    bool
	}{
		{"correct", true},
		{"incorrect", true},
		{"uncertain", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.feedback, func(t *testing.T) {
			result := IsValidFeedback(tt.feedback)
			assert.Equal(t, tt.valid, result)
		})
	}
}

func TestValidation_CorrectedCategory(t *testing.T) {
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
		Category:     "Wrong Category",
	}
	db.Create(classification)

	// User says it's incorrect and provides correct category
	validation := &Validation{
		BatchID:           batch.ID,
		ClassificationID:  classification.ID,
		UserFeedback:      "incorrect",
		CorrectedCategory: "Correct Category",
		UserNotes:         "Should be marketing, not sales",
	}
	err := db.Create(validation).Error
	assert.NoError(t, err)

	// Verify data
	var loaded Validation
	db.First(&loaded, validation.ID)
	assert.Equal(t, "incorrect", loaded.UserFeedback)
	assert.Equal(t, "Correct Category", loaded.CorrectedCategory)
	assert.Equal(t, "Should be marketing, not sales", loaded.UserNotes)
}