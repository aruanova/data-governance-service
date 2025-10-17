package parsers

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

// CSVParser parses CSV files
type CSVParser struct {
	config *ParserConfig
}

// NewCSVParser creates a new CSV parser
func NewCSVParser(config *ParserConfig) *CSVParser {
	if config == nil {
		config = DefaultParserConfig()
	}
	return &CSVParser{
		config: config,
	}
}

// Parse reads and parses a CSV file from disk
func (p *CSVParser) Parse(ctx context.Context, filePath string) (*ParseResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
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

// ParseStream reads and parses CSV data from an io.Reader
func (p *CSVParser) ParseStream(ctx context.Context, reader interface{}) (*ParseResult, error) {
	r, ok := reader.(io.Reader)
	if !ok {
		return nil, fmt.Errorf("reader must implement io.Reader")
	}

	csvReader := csv.NewReader(r)
	csvReader.TrimLeadingSpace = p.config.TrimWhitespace
	csvReader.FieldsPerRecord = -1 // Allow variable number of fields per record

	// Read header row
	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Trim column names if configured
	if p.config.TrimWhitespace {
		for i := range header {
			header[i] = strings.TrimSpace(header[i])
		}
	}

	records := make([]Record, 0, p.config.MaxRowsInMemory)
	totalRows := 0
	skippedRows := 0

	// Read data rows
	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Skip malformed rows but continue parsing
			totalRows++
			skippedRows++
			continue
		}

		totalRows++

		// Check if row is empty
		if p.config.SkipEmptyRows && isEmptyRow(row) {
			skippedRows++
			continue
		}

		// Convert row to Record
		record := make(Record)
		for i, col := range header {
			if i < len(row) {
				value := row[i]
				if p.config.TrimWhitespace {
					value = strings.TrimSpace(value)
				}
				record[col] = value
			} else {
				// Handle missing columns
				record[col] = ""
			}
		}

		records = append(records, record)
	}

	return &ParseResult{
		Records:     records,
		TotalRows:   totalRows,
		SkippedRows: skippedRows,
		Columns:     header,
		Format:      "CSV",
	}, nil
}

// SupportedFormats returns the file extensions this parser supports
func (p *CSVParser) SupportedFormats() []string {
	return []string{".csv"}
}

// isEmptyRow checks if a row contains only empty strings
func isEmptyRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}