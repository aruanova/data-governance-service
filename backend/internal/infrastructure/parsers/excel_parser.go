package parsers

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ExcelParser parses Excel files (.xlsx, .xls)
type ExcelParser struct {
	config *ParserConfig
}

// NewExcelParser creates a new Excel parser
func NewExcelParser(config *ParserConfig) *ExcelParser {
	if config == nil {
		config = DefaultParserConfig()
	}
	return &ExcelParser{
		config: config,
	}
}

// Parse reads and parses an Excel file from disk
func (p *ExcelParser) Parse(ctx context.Context, filePath string) (*ParseResult, error) {
	// Check file size if limit is set
	if p.config.MaxFileSize > 0 {
		stat, err := os.Stat(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat file: %w", err)
		}
		if stat.Size() > p.config.MaxFileSize {
			return nil, fmt.Errorf("file size %d exceeds maximum %d", stat.Size(), p.config.MaxFileSize)
		}
	}

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	return p.parseExcelFile(ctx, f)
}

// ParseStream reads and parses Excel data from an io.Reader
func (p *ExcelParser) ParseStream(ctx context.Context, reader interface{}) (*ParseResult, error) {
	r, ok := reader.(io.Reader)
	if !ok {
		return nil, fmt.Errorf("reader must implement io.Reader")
	}

	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read Excel stream: %w", err)
	}
	defer f.Close()

	return p.parseExcelFile(ctx, f)
}

// parseExcelFile extracts data from the first sheet of an Excel file
func (p *ExcelParser) parseExcelFile(ctx context.Context, f *excelize.File) (*ParseResult, error) {
	// Get the first sheet
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("no sheets found in Excel file")
	}

	// Get all rows from the first sheet
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows from sheet %s: %w", sheetName, err)
	}

	if len(rows) == 0 {
		return &ParseResult{
			Records:     []Record{},
			TotalRows:   0,
			SkippedRows: 0,
			Columns:     []string{},
			Format:      "XLSX",
		}, nil
	}

	// Extract header (first row)
	header := rows[0]
	if p.config.TrimWhitespace {
		for i := range header {
			header[i] = strings.TrimSpace(header[i])
		}
	}

	records := make([]Record, 0, len(rows)-1)
	totalRows := 0
	skippedRows := 0

	// Process data rows (skip header)
	for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		row := rows[rowIdx]
		totalRows++

		// Check if row is empty
		if p.config.SkipEmptyRows && isEmptyRow(row) {
			skippedRows++
			continue
		}

		// Convert row to Record
		record := make(Record)
		for i, colName := range header {
			if i < len(row) {
				value := row[i]
				if p.config.TrimWhitespace {
					value = strings.TrimSpace(value)
				}
				record[colName] = value
			} else {
				// Handle missing columns
				record[colName] = ""
			}
		}

		records = append(records, record)
	}

	return &ParseResult{
		Records:     records,
		TotalRows:   totalRows,
		SkippedRows: skippedRows,
		Columns:     header,
		Format:      "XLSX",
	}, nil
}

// SupportedFormats returns the file extensions this parser supports
func (p *ExcelParser) SupportedFormats() []string {
	return []string{".xlsx", ".xls"}
}
