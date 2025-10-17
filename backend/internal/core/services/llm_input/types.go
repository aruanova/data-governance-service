package llm_input

import (
	"time"

	"github.com/google/uuid"
)

// LLMInputGenerator defines the interface for generating LLM-optimized JSON
type LLMInputGenerator interface {
	GenerateInput(records []Record, config GeneratorConfig) (*LLMInput, error)
	DetectCleanFields(record Record) []string
	EstimateTokenCount(input *LLMInput) int
}

// Record represents a single data record with clean fields
type Record struct {
	RowIndex     int                    `json:"_row_index"`
	OriginalData map[string]interface{} `json:"original_data,omitempty"`
	CleanedData  map[string]interface{} `json:"cleaned_data"`
}

// GeneratorConfig contains configuration for JSON generation
type GeneratorConfig struct {
	// Only include clean fields (reduces token count)
	OnlyCleanFields bool `json:"only_clean_fields"`

	// Include metadata context
	IncludeMetadata bool `json:"include_metadata"`

	// Maximum records per chunk
	ChunkSize int `json:"chunk_size"`

	// Fields to include (if empty, auto-detect clean* fields)
	FieldsToInclude []string `json:"fields_to_include,omitempty"`

	// Compact mode: minimal whitespace
	CompactMode bool `json:"compact_mode"`
}

// LLMInput represents the optimized JSON structure for LLM processing
type LLMInput struct {
	// Metadata about the batch
	Metadata InputMetadata `json:"metadata"`

	// The actual records to classify
	Records []CleanRecord `json:"records"`

	// Statistics about the input
	Stats InputStats `json:"stats"`
}

// InputMetadata contains context about the data
type InputMetadata struct {
	BatchID      uuid.UUID `json:"batch_id"`
	TotalRecords int       `json:"total_records"`
	ChunkNumber  int       `json:"chunk_number,omitempty"`
	TotalChunks  int       `json:"total_chunks,omitempty"`
	Fields       []string  `json:"fields"`
	GeneratedAt  time.Time `json:"generated_at"`
	Version      string    `json:"version"`
}

// CleanRecord represents a single record with only clean fields
type CleanRecord struct {
	RowIndex int                    `json:"_row_index"`
	Data     map[string]interface{} `json:"data"`
}

// InputStats provides statistics about the generated input
type InputStats struct {
	TotalRecords       int     `json:"total_records"`
	EstimatedTokens    int     `json:"estimated_tokens"`
	AvgFieldsPerRecord float64 `json:"avg_fields_per_record"`
	CleanFieldsUsed    []string `json:"clean_fields_used"`
}

// DefaultGeneratorConfig returns a configuration optimized for token efficiency
func DefaultGeneratorConfig() GeneratorConfig {
	return GeneratorConfig{
		OnlyCleanFields: true,
		IncludeMetadata: true,
		ChunkSize:       100, // Default chunk size
		CompactMode:     true,
	}
}

// WithChunkSize creates a config with custom chunk size
func (c GeneratorConfig) WithChunkSize(size int) GeneratorConfig {
	c.ChunkSize = size
	return c
}

// WithFields creates a config with specific fields
func (c GeneratorConfig) WithFields(fields []string) GeneratorConfig {
	c.FieldsToInclude = fields
	return c
}

// WithMetadata enables/disables metadata inclusion
func (c GeneratorConfig) WithMetadata(include bool) GeneratorConfig {
	c.IncludeMetadata = include
	return c
}