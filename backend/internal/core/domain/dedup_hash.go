package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DedupHash tracks deduplication hashes for records
type DedupHash struct {
	ID               uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	BatchID          uuid.UUID `gorm:"type:uuid;not null;index:idx_dedup_batch_hash" json:"batch_id"`
	Hash             string    `gorm:"type:varchar(64);not null;index:idx_dedup_batch_hash" json:"hash"`
	OriginalRowIndex int       `gorm:"not null" json:"original_row_index"`
	Kept             bool      `gorm:"default:true;index:idx_dedup_kept" json:"kept"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relations
	Batch *Batch `gorm:"foreignKey:BatchID" json:"batch,omitempty"`
}

// TableName specifies the table name for GORM
func (DedupHash) TableName() string {
	return "dedup_hashes"
}

// BeforeCreate GORM hook
func (d *DedupHash) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}