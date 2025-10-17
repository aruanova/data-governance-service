package parsers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// JSONParser parses JSON files
type JSONParser struct {
	config *ParserConfig
}

// NewJSONParser creates a new JSON parser
func NewJSONParser(config *ParserConfig) *JSONParser {
	if config == nil {
		config = DefaultParserConfig()
	}
	return &JSONParser{
		config: config,
	}
}

// Parse reads and parses a JSON file from disk
func (p *JSONParser) Parse(ctx context.Context, filePath string) (*ParseResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open JSON file: %w", err)
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

// ParseStream reads and parses JSON data from an io.Reader
func (p *JSONParser) ParseStream(ctx context.Context, reader interface{}) (*ParseResult, error) {
	r, ok := reader.(io.Reader)
	if !ok {
		return nil, fmt.Errorf("reader must implement io.Reader")
	}

	// Try to parse as array of objects first
	var records []Record
	decoder := json.NewDecoder(r)

	// Peek at the first token to determine structure
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON: %w", err)
	}

	// Check if it's an array
	if delim, ok := token.(json.Delim); ok && delim == '[' {
		// Parse array of objects
		for decoder.More() {
			// Check context cancellation
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			var record Record
			if err := decoder.Decode(&record); err != nil {
				return nil, fmt.Errorf("failed to decode JSON record: %w", err)
			}
			records = append(records, record)
		}

		// Read the closing bracket
		if _, err := decoder.Token(); err != nil {
			return nil, fmt.Errorf("failed to read closing bracket: %w", err)
		}
	} else {
		// Single object - wrap in array
		// We need to rewind, so read the whole thing
		r, ok := reader.(io.ReadSeeker)
		if !ok {
			return nil, fmt.Errorf("cannot parse single JSON object from non-seekable stream")
		}
		if _, err := r.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to rewind stream: %w", err)
		}

		var record Record
		decoder = json.NewDecoder(r)
		if err := decoder.Decode(&record); err != nil {
			return nil, fmt.Errorf("failed to decode JSON object: %w", err)
		}
		records = []Record{record}
	}

	// Extract column names from first record
	var columns []string
	if len(records) > 0 {
		columns = make([]string, 0, len(records[0]))
		for key := range records[0] {
			columns = append(columns, key)
		}
	}

	return &ParseResult{
		Records:     records,
		TotalRows:   len(records),
		SkippedRows: 0,
		Columns:     columns,
		Format:      "JSON",
	}, nil
}

// SupportedFormats returns the file extensions this parser supports
func (p *JSONParser) SupportedFormats() []string {
	return []string{".json"}
}