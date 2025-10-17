package llm_input

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Generator implements the LLMInputGenerator interface
type Generator struct {
	logger *slog.Logger
}

// NewGenerator creates a new LLM input generator
func NewGenerator(logger *slog.Logger) *Generator {
	if logger == nil {
		logger = slog.Default()
	}

	return &Generator{
		logger: logger,
	}
}

// GenerateInput creates optimized JSON input for LLM processing
func (g *Generator) GenerateInput(records []Record, config GeneratorConfig) (*LLMInput, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("no records provided")
	}

	// Detect clean fields if not specified
	fieldsToInclude := config.FieldsToInclude
	if len(fieldsToInclude) == 0 {
		fieldsToInclude = g.DetectCleanFields(records[0])
	}

	if len(fieldsToInclude) == 0 {
		return nil, fmt.Errorf("no clean fields detected")
	}

	g.logger.Info("generating LLM input",
		slog.Int("record_count", len(records)),
		slog.Int("field_count", len(fieldsToInclude)),
		slog.Bool("only_clean_fields", config.OnlyCleanFields))

	// Build clean records
	cleanRecords := make([]CleanRecord, 0, len(records))
	totalFields := 0

	for _, record := range records {
		cleanData := make(map[string]interface{})

		// Extract only the specified fields
		dataSource := record.CleanedData
		if !config.OnlyCleanFields && len(record.OriginalData) > 0 {
			dataSource = record.OriginalData
		}

		for _, field := range fieldsToInclude {
			if value, exists := dataSource[field]; exists {
				cleanData[field] = value
				totalFields++
			}
		}

		// Skip records with no data
		if len(cleanData) == 0 {
			g.logger.Warn("skipping record with no clean data",
				slog.Int("row_index", record.RowIndex))
			continue
		}

		cleanRecords = append(cleanRecords, CleanRecord{
			RowIndex: record.RowIndex,
			Data:     cleanData,
		})
	}

	// Build metadata
	batchID := uuid.New()
	metadata := InputMetadata{
		BatchID:      batchID,
		TotalRecords: len(cleanRecords),
		Fields:       fieldsToInclude,
		GeneratedAt:  time.Now(),
		Version:      "1.0",
	}

	// Build the complete input
	input := &LLMInput{
		Metadata: metadata,
		Records:  cleanRecords,
	}

	// Calculate statistics
	avgFields := 0.0
	if len(cleanRecords) > 0 {
		avgFields = float64(totalFields) / float64(len(cleanRecords))
	}

	input.Stats = InputStats{
		TotalRecords:       len(cleanRecords),
		EstimatedTokens:    g.EstimateTokenCount(input),
		AvgFieldsPerRecord: avgFields,
		CleanFieldsUsed:    fieldsToInclude,
	}

	g.logger.Info("LLM input generated",
		slog.Int("clean_records", len(cleanRecords)),
		slog.Int("estimated_tokens", input.Stats.EstimatedTokens),
		slog.Float64("avg_fields", avgFields))

	return input, nil
}

// DetectCleanFields automatically detects fields starting with "clean"
func (g *Generator) DetectCleanFields(record Record) []string {
	cleanFields := make([]string, 0)

	// Check cleaned data first
	for field := range record.CleanedData {
		if strings.HasPrefix(strings.ToLower(field), "clean") {
			cleanFields = append(cleanFields, field)
		}
	}

	// If no clean fields in CleanedData, check OriginalData
	if len(cleanFields) == 0 {
		for field := range record.OriginalData {
			if strings.HasPrefix(strings.ToLower(field), "clean") {
				cleanFields = append(cleanFields, field)
			}
		}
	}

	g.logger.Debug("detected clean fields",
		slog.Int("count", len(cleanFields)),
		slog.Any("fields", cleanFields))

	return cleanFields
}

// EstimateTokenCount provides a rough estimate of token count
// Based on the rule: ~4 characters per token for English/Spanish text
func (g *Generator) EstimateTokenCount(input *LLMInput) int {
	// Serialize to JSON to get accurate character count
	jsonBytes, err := json.Marshal(input)
	if err != nil {
		g.logger.Warn("failed to marshal for token estimation", "error", err)
		return 0
	}

	// Rough estimation: 1 token ≈ 4 characters
	charCount := len(jsonBytes)
	estimatedTokens := charCount / 4

	// Add buffer for prompt overhead (instructions, examples, etc.)
	// Typically adds 200-500 tokens depending on prompt complexity
	promptOverhead := 300
	totalEstimate := estimatedTokens + promptOverhead

	g.logger.Debug("token estimation",
		slog.Int("char_count", charCount),
		slog.Int("data_tokens", estimatedTokens),
		slog.Int("total_tokens", totalEstimate))

	return totalEstimate
}

// GenerateChunks splits records into multiple LLM inputs
func (g *Generator) GenerateChunks(records []Record, config GeneratorConfig) ([]*LLMInput, error) {
	if config.ChunkSize <= 0 {
		return nil, fmt.Errorf("chunk_size must be greater than 0")
	}

	totalRecords := len(records)
	totalChunks := (totalRecords + config.ChunkSize - 1) / config.ChunkSize

	g.logger.Info("generating chunks",
		slog.Int("total_records", totalRecords),
		slog.Int("chunk_size", config.ChunkSize),
		slog.Int("total_chunks", totalChunks))

	chunks := make([]*LLMInput, 0, totalChunks)

	for i := 0; i < totalChunks; i++ {
		start := i * config.ChunkSize
		end := start + config.ChunkSize
		if end > totalRecords {
			end = totalRecords
		}

		chunkRecords := records[start:end]

		input, err := g.GenerateInput(chunkRecords, config)
		if err != nil {
			return nil, fmt.Errorf("failed to generate chunk %d: %w", i, err)
		}

		// Update metadata with chunk info
		input.Metadata.ChunkNumber = i + 1
		input.Metadata.TotalChunks = totalChunks

		chunks = append(chunks, input)
	}

	g.logger.Info("chunks generated successfully",
		slog.Int("chunk_count", len(chunks)))

	return chunks, nil
}

// ToJSON serializes the LLM input to JSON
func (g *Generator) ToJSON(input *LLMInput, compact bool) ([]byte, error) {
	if compact {
		return json.Marshal(input)
	}
	return json.MarshalIndent(input, "", "  ")
}

// ToJSONString serializes to JSON string
func (g *Generator) ToJSONString(input *LLMInput, compact bool) (string, error) {
	jsonBytes, err := g.ToJSON(input, compact)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// ValidateInput checks if the generated input is valid
func (g *Generator) ValidateInput(input *LLMInput) error {
	if input == nil {
		return fmt.Errorf("input is nil")
	}

	if len(input.Records) == 0 {
		return fmt.Errorf("no records in input")
	}

	if len(input.Metadata.Fields) == 0 {
		return fmt.Errorf("no fields specified in metadata")
	}

	// Check for row_index uniqueness within this input
	rowIndices := make(map[int]bool)
	for _, record := range input.Records {
		if rowIndices[record.RowIndex] {
			return fmt.Errorf("duplicate row_index: %d", record.RowIndex)
		}
		rowIndices[record.RowIndex] = true

		if len(record.Data) == 0 {
			return fmt.Errorf("record at row_index %d has no data", record.RowIndex)
		}
	}

	g.logger.Debug("input validation passed",
		slog.Int("record_count", len(input.Records)))

	return nil
}

// BuildRecordFromMap creates a Record from a map (helper for integration)
func BuildRecordFromMap(rowIndex int, originalData, cleanedData map[string]interface{}) Record {
	return Record{
		RowIndex:     rowIndex,
		OriginalData: originalData,
		CleanedData:  cleanedData,
	}
}

// ExtractCleanFields extracts only clean* fields from a map
func ExtractCleanFields(data map[string]interface{}) map[string]interface{} {
	clean := make(map[string]interface{})
	for key, value := range data {
		if strings.HasPrefix(strings.ToLower(key), "clean") {
			clean[key] = value
		}
	}
	return clean
}