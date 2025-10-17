package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Validation represents a manual validation sample
type Validation struct {
	ID                uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	BatchID           uuid.UUID  `gorm:"type:uuid;not null;index:idx_validations_batch" json:"batch_id"`
	ClassificationID  uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_unique_classification_validation" json:"classification_id"`
	SamplingStrategy  string     `gorm:"type:varchar(100)" json:"sampling_strategy"`
	UserFeedback      string     `gorm:"type:varchar(50)" json:"user_feedback"` // correct, incorrect, uncertain
	CorrectedCategory string     `gorm:"type:varchar(255)" json:"corrected_category,omitempty"`
	UserNotes         string     `gorm:"type:text" json:"user_notes,omitempty"`
	IdempotencyKey    string     `gorm:"type:varchar(64);uniqueIndex:idx_validations_idempotency" json:"idempotency_key,omitempty"` // For API idempotency
	ValidatedAt       time.Time  `gorm:"autoCreateTime" json:"validated_at"`

	// Relations
	Batch          *Batch          `gorm:"foreignKey:BatchID" json:"batch,omitempty"`
	Classification *Classification `gorm:"foreignKey:ClassificationID" json:"classification,omitempty"`
}

// TableName specifies the table name for GORM
func (Validation) TableName() string {
	return "validations"
}

// BeforeCreate GORM hook
func (v *Validation) BeforeCreate(tx *gorm.DB) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	return nil
}

// ValidFeedbacks returns list of valid feedback values
func ValidFeedbacks() []string {
	return []string{"correct", "incorrect", "uncertain"}
}

// IsValidFeedback checks if feedback is valid
func IsValidFeedback(feedback string) bool {
	for _, f := range ValidFeedbacks() {
		if f == feedback {
			return true
		}
	}
	return false
}