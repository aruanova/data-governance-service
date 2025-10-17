package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Iteration tracks prompt refinement iterations
type Iteration struct {
	ID              uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	BatchID         uuid.UUID  `gorm:"type:uuid;not null;index:idx_iterations_batch" json:"batch_id"`
	IterationNumber int        `gorm:"not null" json:"iteration_number"`
	PromptID        *uuid.UUID `gorm:"type:uuid" json:"prompt_id,omitempty"`
	PromptChanges   string     `gorm:"type:text" json:"prompt_changes,omitempty"`
	Metrics         JSONB      `gorm:"type:jsonb" json:"metrics,omitempty"`
	AccuracyDelta   *float64   `gorm:"type:decimal(5,2)" json:"accuracy_delta,omitempty"`
	CreatedAt       time.Time  `gorm:"autoCreateTime" json:"created_at"`

	// Relations
	Batch  *Batch  `gorm:"foreignKey:BatchID" json:"batch,omitempty"`
	Prompt *Prompt `gorm:"foreignKey:PromptID" json:"prompt,omitempty"`
}

// TableName specifies the table name for GORM
func (Iteration) TableName() string {
	return "iterations"
}

// BeforeCreate GORM hook
func (i *Iteration) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return nil
}

// Note: Unique index on (batch_id, iteration_number) is created via SQL migration
// to ensure one iteration number per batch