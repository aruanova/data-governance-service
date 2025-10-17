package deduplication

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// Strategy defines the deduplication strategy
type Strategy string

const (
	StrategyExact     Strategy = "exact"      // Exact match within batch
	StrategyFuzzy     Strategy = "fuzzy"      // Fuzzy matching (normalized)
	StrategyUniversal Strategy = "universal"  // Cross-session deduplication
)

// Record represents a data record to be deduplicated
type Record struct {
	RowIndex int                    `json:"row_index"`
	Data     map[string]interface{} `json:"data"`
	Hash     string                 `json:"hash,omitempty"`
}

// DeduplicationResult contains the result of deduplication
type DeduplicationResult struct {
	OriginalCount    int                `json:"original_count"`
	DeduplicatedCount int               `json:"deduplicated_count"`
	RemovedCount     int                `json:"removed_count"`
	Strategy         Strategy           `json:"strategy"`
	Records          []Record           `json:"records"`
	Stats            DeduplicationStats `json:"stats"`
}

// DeduplicationStats provides detailed statistics
type DeduplicationStats struct {
	Level1Duplicates int            `json:"level1_duplicates"` // Within batch
	Level2Duplicates int            `json:"level2_duplicates"` // Cross-session
	UniqueRecords    int            `json:"unique_records"`
	ProcessingTimeMs int64          `json:"processing_time_ms"`
	HashDistribution map[string]int `json:"hash_distribution,omitempty"`
}

// Config for deduplication service
type Config struct {
	Strategy       Strategy `json:"strategy"`
	CleanFields    []string `json:"clean_fields"`     // Fields to use for hashing
	EnableLevel2   bool     `json:"enable_level2"`    // Enable cross-session dedup
	StoreHashes    bool     `json:"store_hashes"`     // Store hashes in DB
	CaseSensitive  bool     `json:"case_sensitive"`   // Case-sensitive comparison
	TrimWhitespace bool     `json:"trim_whitespace"`  // Trim whitespace before hashing
}

// DefaultConfig returns default deduplication configuration
func DefaultConfig() Config {
	return Config{
		Strategy:       StrategyExact,
		CleanFields:    []string{"cleanLineDescription"},
		EnableLevel2:   false,
		StoreHashes:    true,
		CaseSensitive:  false,
		TrimWhitespace: true,
	}
}

// HashRepository defines the interface for hash storage
type HashRepository interface {
	// CheckHashExists verifies if a hash exists for any batch (universal dedup)
	CheckHashExists(ctx context.Context, hash string) (bool, error)

	// SaveHashes stores deduplication hashes for a batch
	SaveHashes(ctx context.Context, batchID uuid.UUID, hashes []HashEntry) error

	// GetBatchHashes retrieves all hashes for a specific batch
	GetBatchHashes(ctx context.Context, batchID uuid.UUID) ([]HashEntry, error)
}

// HashEntry represents a hash entry to be stored
type HashEntry struct {
	Hash             string
	OriginalRowIndex int
	Kept             bool
}

// Deduplicator defines the interface for deduplication operations
type Deduplicator interface {
	// Deduplicate performs deduplication on a set of records
	Deduplicate(ctx context.Context, batchID uuid.UUID, records []Record) (*DeduplicationResult, error)

	// GetConfig returns the current configuration
	GetConfig() Config
}

// generateHash creates a SHA256 hash from record data
func generateHash(record Record, fields []string, config Config) (string, error) {
	// Extract only specified fields for hashing
	hashData := make(map[string]interface{})

	for _, field := range fields {
		if val, exists := record.Data[field]; exists {
			// Normalize value based on config
			normalized := normalizeValue(val, config)
			hashData[field] = normalized
		}
	}

	// Marshal to JSON for consistent hashing
	jsonData, err := json.Marshal(hashData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal hash data: %w", err)
	}

	// Generate SHA256 hash
	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:]), nil
}

// normalizeValue normalizes a value based on configuration
func normalizeValue(val interface{}, config Config) interface{} {
	strVal, ok := val.(string)
	if !ok {
		return val
	}

	// Trim whitespace if configured
	if config.TrimWhitespace {
		strVal = trimWhitespace(strVal)
	}

	// Convert to lowercase if not case-sensitive
	if !config.CaseSensitive {
		strVal = toLowerCase(strVal)
	}

	return strVal
}

// Helper functions
func trimWhitespace(s string) string {
	// Simple trim implementation
	start := 0
	end := len(s)

	for start < end && isWhitespace(s[start]) {
		start++
	}

	for end > start && isWhitespace(s[end-1]) {
		end--
	}

	return s[start:end]
}

func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func toLowerCase(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}