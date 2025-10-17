package parsers

import "context"

// Record represents a single data record as a map
type Record map[string]interface{}

// ParseResult contains parsing statistics
type ParseResult struct {
	Records      []Record
	TotalRows    int
	SkippedRows  int
	Columns      []string
	Format       string
	ParsingError error
}

// FileParser is the interface all parsers must implement
type FileParser interface {
	// Parse reads and parses the file from the given path
	Parse(ctx context.Context, filePath string) (*ParseResult, error)

	// ParseStream reads and parses from an io.Reader
	ParseStream(ctx context.Context, reader interface{}) (*ParseResult, error)

	// SupportedFormats returns the file extensions this parser supports
	SupportedFormats() []string
}

// ParserConfig holds configuration for all parsers
type ParserConfig struct {
	// MaxRowsInMemory limits how many rows to keep in memory at once (for streaming)
	MaxRowsInMemory int

	// SkipEmptyRows determines if empty rows should be skipped
	SkipEmptyRows bool

	// TrimWhitespace determines if cell values should be trimmed
	TrimWhitespace bool

	// MaxFileSize is the maximum file size in bytes (0 = unlimited)
	MaxFileSize int64
}

// DefaultParserConfig returns sensible defaults
func DefaultParserConfig() *ParserConfig {
	return &ParserConfig{
		MaxRowsInMemory: 10000,
		SkipEmptyRows:   true,
		TrimWhitespace:  true,
		MaxFileSize:     500 * 1024 * 1024, // 500 MB
	}
}