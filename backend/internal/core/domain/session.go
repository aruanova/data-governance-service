package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Session represents user workflow state
type Session struct {
	ID           uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	BatchID      *uuid.UUID `gorm:"type:uuid;index:idx_sessions_batch" json:"batch_id,omitempty"`
	UserID       string     `gorm:"type:varchar(255);index:idx_sessions_user" json:"user_id,omitempty"`
	CurrentStep  string     `gorm:"type:varchar(50);not null;default:'upload'" json:"current_step"`
	State        JSONB      `gorm:"type:jsonb" json:"state,omitempty"`
	LastActivity time.Time  `gorm:"autoUpdateTime" json:"last_activity"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
	ExpiresAt    *time.Time `gorm:"index:idx_sessions_expires" json:"expires_at,omitempty"`

	// Relations
	Batch *Batch `gorm:"foreignKey:BatchID" json:"batch,omitempty"`
}

// TableName specifies the table name for GORM
func (Session) TableName() string {
	return "sessions"
}

// BeforeCreate GORM hook
func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	// Set default expiration to 24 hours if not set
	if s.ExpiresAt == nil {
		expiresAt := time.Now().Add(24 * time.Hour)
		s.ExpiresAt = &expiresAt
	}
	return nil
}

// IsExpired checks if the session has expired
func (s *Session) IsExpired() bool {
	if s.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*s.ExpiresAt)
}
