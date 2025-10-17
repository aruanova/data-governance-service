package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Classification represents a single LLM classification result
type Classification struct {
	ID                uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	BatchID           uuid.UUID  `gorm:"type:uuid;not null;index:idx_classifications_batch" json:"batch_id"`
	RowIndex          int        `gorm:"not null" json:"row_index"`
	OriginalData      JSONB      `gorm:"type:jsonb;not null" json:"original_data"`
	CleanedData       JSONB      `gorm:"type:jsonb;not null" json:"cleaned_data"`
	Category          string     `gorm:"type:varchar(255);index:idx_classifications_category" json:"category"`
	Reason            string     `gorm:"type:text" json:"reason"`
	ConfidenceScore   *float64   `gorm:"type:decimal(5,4);index:idx_classifications_confidence" json:"confidence_score,omitempty"`
	LLMProvider       string     `gorm:"type:varchar(50)" json:"llm_provider"`
	LLMModel          string     `gorm:"type:varchar(100)" json:"llm_model"`
	TokensUsed        int        `json:"tokens_used"`
	ProcessingTimeMs  int        `json:"processing_time_ms"`
	CreatedAt         time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime" json:"updated_at"`

	// Relations
	Batch             *Batch       `gorm:"foreignKey:BatchID" json:"batch,omitempty"`
	Validations       []Validation `gorm:"foreignKey:ClassificationID;constraint:OnDelete:CASCADE" json:"validations,omitempty"`
}

// TableName specifies the table name for GORM
func (Classification) TableName() string {
	return "classifications"
}

// BeforeCreate GORM hook
func (c *Classification) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// Note: Unique index on (batch_id, row_index) is created via SQL migration
// for idempotency - ensures one classification per row