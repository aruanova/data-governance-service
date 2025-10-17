package parsers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// JSONLParser parses JSONL/NDJSON files (newline-delimited JSON)
type JSONLParser struct {
	config *ParserConfig
}

// NewJSONLParser creates a new JSONL parser
func NewJSONLParser(config *ParserConfig) *JSONLParser {
	if config == nil {
		config = DefaultParserConfig()
	}
	return &JSONLParser{
		config: config,
	}
}

// Parse reads and parses a JSONL file from disk
func (p *JSONLParser) Parse(ctx context.Context, filePath string) (*ParseResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open JSONL file: %w", err)
	}
	defer file.Close()

	// Check file size if limit is set
	if p.config.MaxFileSize > 0 {
		stat, err := file.Stat()
		if err != nil {
			return nil, fmt.Errorf("failed to stat file: %w", err)
		}
		if stat.Size() > p.config.MaxFileSize {
			return nil, fmt.Errorf("file size %d exceeds maximum %d", stat.Size(), p.config.MaxFileSize)
		}
	}

	return p.ParseStream(ctx, file)
}

// ParseStream reads and parses JSONL data from an io.Reader
func (p *JSONLParser) ParseStream(ctx context.Context, reader interface{}) (*ParseResult, error) {
	r, ok := reader.(io.Reader)
	if !ok {
		return nil, fmt.Errorf("reader must implement io.Reader")
	}

	scanner := bufio.NewScanner(r)
	// Set a larger buffer for potentially large JSON lines (max 1MB per line)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	records := make([]Record, 0, p.config.MaxRowsInMemory)
	var columns []string
	columnSet := make(map[string]bool)
	totalRows := 0
	skippedRows := 0

	// Read line by line
	for scanner.Scan() {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		line := scanner.Bytes()
		totalRows++

		// Skip empty lines
		if len(line) == 0 {
			skippedRows++
			continue
		}

		// Parse JSON object
		var record Record
		if err := json.Unmarshal(line, &record); err != nil {
			// Skip malformed JSON lines but continue parsing
			skippedRows++
			continue
		}

		// Check if record is empty
		if p.config.SkipEmptyRows && len(record) == 0 {
			skippedRows++
			continue
		}

		// Collect all unique column names
		for key := range record {
			if !columnSet[key] {
				columnSet[key] = true
				columns = append(columns, key)
			}
		}

		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading JSONL stream: %w", err)
	}

	return &ParseResult{
		Records:     records,
		TotalRows:   totalRows,
		SkippedRows: skippedRows,
		Columns:     columns,
		Format:      "JSONL",
	}, nil
}

// SupportedFormats returns the file extensions this parser supports
func (p *JSONLParser) SupportedFormats() []string {
	return []string{".jsonl", ".ndjson", ".jsonnl"}
}