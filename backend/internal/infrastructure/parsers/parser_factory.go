package parsers

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// ParserFactory creates the appropriate parser based on file extension
type ParserFactory struct {
	config  *ParserConfig
	parsers map[string]FileParser
}

// NewParserFactory creates a new parser factory with all built-in parsers
func NewParserFactory(config *ParserConfig) *ParserFactory {
	if config == nil {
		config = DefaultParserConfig()
	}

	factory := &ParserFactory{
		config:  config,
		parsers: make(map[string]FileParser),
	}

	// Register built-in parsers
	factory.RegisterParser(NewCSVParser(config))
	factory.RegisterParser(NewExcelParser(config))
	factory.RegisterParser(NewJSONParser(config))
	factory.RegisterParser(NewJSONLParser(config))

	return factory
}

// RegisterParser registers a custom parser
func (f *ParserFactory) RegisterParser(parser FileParser) {
	for _, ext := range parser.SupportedFormats() {
		ext = strings.ToLower(ext)
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		f.parsers[ext] = parser
	}
}

// GetParser returns the appropriate parser for a file extension
func (f *ParserFactory) GetParser(fileExt string) (FileParser, error) {
	ext := strings.ToLower(fileExt)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	parser, exists := f.parsers[ext]
	if !exists {
		return nil, fmt.Errorf("no parser found for extension: %s", fileExt)
	}

	return parser, nil
}

// GetParserForFile returns the appropriate parser based on file path
func (f *ParserFactory) GetParserForFile(filePath string) (FileParser, error) {
	ext := filepath.Ext(filePath)
	return f.GetParser(ext)
}

// ParseFile is a convenience method that automatically selects and uses the correct parser
func (f *ParserFactory) ParseFile(ctx context.Context, filePath string) (*ParseResult, error) {
	parser, err := f.GetParserForFile(filePath)
	if err != nil {
		return nil, err
	}

	return parser.Parse(ctx, filePath)
}

// SupportedFormats returns all supported file extensions
func (f *ParserFactory) SupportedFormats() []string {
	formats := make([]string, 0, len(f.parsers))
	for ext := range f.parsers {
		formats = append(formats, ext)
	}
	return formats
}

// IsSupported checks if a file extension is supported
func (f *ParserFactory) IsSupported(fileExt string) bool {
	ext := strings.ToLower(fileExt)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	_, exists := f.parsers[ext]
	return exists
}