package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Batch represents a file processing session
type Batch struct {
	ID                uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OriginalFilename  string         `gorm:"type:varchar(500);not null" json:"original_filename"`
	FilePath          string         `gorm:"type:text" json:"file_path"`
	FileHash          string         `gorm:"type:varchar(64);uniqueIndex;not null" json:"file_hash"` // For idempotency
	Status            string         `gorm:"type:varchar(50);not null;default:'uploaded'" json:"status"`
	TotalRecords      int            `gorm:"default:0" json:"total_records"`
	ProcessedRecords  int            `gorm:"default:0" json:"processed_records"`
	Config            JSONB          `gorm:"type:jsonb" json:"config"`
	Metadata          JSONB          `gorm:"type:jsonb" json:"metadata"`
	CreatedAt         time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	CompletedAt       *time.Time     `json:"completed_at,omitempty"`

	// Relations
	Classifications   []Classification `gorm:"foreignKey:BatchID;constraint:OnDelete:CASCADE" json:"classifications,omitempty"`
	Validations       []Validation     `gorm:"foreignKey:BatchID;constraint:OnDelete:CASCADE" json:"validations,omitempty"`
	Iterations        []Iteration      `gorm:"foreignKey:BatchID;constraint:OnDelete:CASCADE" json:"iterations,omitempty"`
	Sessions          []Session        `gorm:"foreignKey:BatchID;constraint:OnDelete:CASCADE" json:"sessions,omitempty"`
	DedupHashes       []DedupHash      `gorm:"foreignKey:BatchID;constraint:OnDelete:CASCADE" json:"dedup_hashes,omitempty"`
}

// TableName specifies the table name for GORM
func (Batch) TableName() string {
	return "batches"
}

// BeforeCreate GORM hook - called before creating a record
func (b *Batch) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

// ValidStatuses returns list of valid batch statuses
func ValidStatuses() []string {
	return []string{
		"uploaded",
		"cleaning",
		"llm_processing",
		"validating",
		"completed",
		"failed",
	}
}

// IsValidStatus checks if a status is valid
func IsValidStatus(status string) bool {
	for _, s := range ValidStatuses() {
		if s == status {
			return true
		}
	}
	return false
}

// JSONB is a custom type for JSONB columns
type JSONB map[string]interface{}
