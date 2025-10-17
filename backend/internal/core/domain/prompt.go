package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Prompt represents a customizable LLM prompt
type Prompt struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name       string    `gorm:"type:varchar(255);not null" json:"name"`
	Label      string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"label"`
	Template   string    `gorm:"type:text;not null" json:"template"`
	Categories JSONB     `gorm:"type:jsonb;not null" json:"categories"`
	IsDefault  bool      `gorm:"default:false;index:idx_prompts_default,where:is_default = true" json:"is_default"`
	CreatedBy  string    `gorm:"type:varchar(255)" json:"created_by"`
	Version    int       `gorm:"default:1" json:"version"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Relations
	Iterations []Iteration `gorm:"foreignKey:PromptID" json:"iterations,omitempty"`
}

// TableName specifies the table name for GORM
func (Prompt) TableName() string {
	return "prompts"
}

// BeforeCreate GORM hook
func (p *Prompt) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// Category represents a classification category within a prompt
type Category struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Priority    int      `json:"priority"`
	Keywords    []string `json:"keywords"`
}