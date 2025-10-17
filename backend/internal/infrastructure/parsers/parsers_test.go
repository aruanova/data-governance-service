package parsers

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestFiles(t *testing.T) string {
	tempDir := t.TempDir()

	// Create test CSV file
	csvContent := `Name,Age,City
John Doe,30,New York
Jane Smith,25,Los Angeles
Bob Johnson,35,Chicago
`
	csvPath := filepath.Join(tempDir, "test.csv")
	require.NoError(t, os.WriteFile(csvPath, []byte(csvContent), 0644))

	// Create test JSON file (array of objects)
	jsonContent := `[
  {"Name": "John Doe", "Age": 30, "City": "New York"},
  {"Name": "Jane Smith", "Age": 25, "City": "Los Angeles"},
  {"Name": "Bob Johnson", "Age": 35, "City": "Chicago"}
]`
	jsonPath := filepath.Join(tempDir, "test.json")
	require.NoError(t, os.WriteFile(jsonPath, []byte(jsonContent), 0644))

	// Create test JSONL file
	jsonlContent := `{"Name": "John Doe", "Age": 30, "City": "New York"}
{"Name": "Jane Smith", "Age": 25, "City": "Los Angeles"}
{"Name": "Bob Johnson", "Age": 35, "City": "Chicago"}
`
	jsonlPath := filepath.Join(tempDir, "test.jsonl")
	require.NoError(t, os.WriteFile(jsonlPath, []byte(jsonlContent), 0644))

	// Create test NDJSON file
	ndjsonPath := filepath.Join(tempDir, "test.ndjson")
	require.NoError(t, os.WriteFile(ndjsonPath, []byte(jsonlContent), 0644))

	// Create test JSONNL file
	jsonnlPath := filepath.Join(tempDir, "test.jsonnl")
	require.NoError(t, os.WriteFile(jsonnlPath, []byte(jsonlContent), 0644))

	return tempDir
}

func TestCSVParser_Parse(t *testing.T) {
	tempDir := setupTestFiles(t)
	csvPath := filepath.Join(tempDir, "test.csv")

	parser := NewCSVParser(nil)
	result, err := parser.Parse(context.Background(), csvPath)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, len(result.Records))
	assert.Equal(t, "CSV", result.Format)
	assert.Equal(t, []string{"Name", "Age", "City"}, result.Columns)

	// Verify first record
	assert.Equal(t, "John Doe", result.Records[0]["Name"])
	assert.Equal(t, "30", result.Records[0]["Age"])
	assert.Equal(t, "New York", result.Records[0]["City"])
}

func TestCSVParser_ParseStream(t *testing.T) {
	csvContent := `Product,Price,Stock
Widget A,10.99,100
Widget B,20.50,50
Widget C,5.25,200
`
	reader := bytes.NewReader([]byte(csvContent))

	parser := NewCSVParser(nil)
	result, err := parser.ParseStream(context.Background(), reader)

	require.NoError(t, err)
	assert.Equal(t, 3, len(result.Records))
	assert.Equal(t, "CSV", result.Format)
	assert.Equal(t, []string{"Product", "Price", "Stock"}, result.Columns)
}

func TestCSVParser_SkipEmptyRows(t *testing.T) {
	csvContent := `Name,Age
John,30
,
Jane,25
,
`
	reader := bytes.NewReader([]byte(csvContent))

	config := DefaultParserConfig()
	config.SkipEmptyRows = true

	parser := NewCSVParser(config)
	result, err := parser.ParseStream(context.Background(), reader)

	require.NoError(t, err)
	assert.Equal(t, 2, len(result.Records)) // Only 2 non-empty rows
	assert.Equal(t, 2, result.SkippedRows)
}

func TestCSVParser_TrimWhitespace(t *testing.T) {
	csvContent := `  Name  ,  Age
  John  ,  30
  Jane  ,  25
`
	reader := bytes.NewReader([]byte(csvContent))

	config := DefaultParserConfig()
	config.TrimWhitespace = true

	parser := NewCSVParser(config)
	result, err := parser.ParseStream(context.Background(), reader)

	require.NoError(t, err)
	assert.Equal(t, []string{"Name", "Age"}, result.Columns)
	assert.Equal(t, "John", result.Records[0]["Name"])
	assert.Equal(t, "30", result.Records[0]["Age"])
}

func TestCSVParser_MissingColumns(t *testing.T) {
	csvContent := `Name,Age,City
John,30,New York
Jane,25
Bob
`
	reader := bytes.NewReader([]byte(csvContent))

	parser := NewCSVParser(nil)
	result, err := parser.ParseStream(context.Background(), reader)

	require.NoError(t, err)
	assert.Equal(t, 3, len(result.Records))

	// Second record missing City
	assert.Equal(t, "", result.Records[1]["City"])

	// Third record missing Age and City
	assert.Equal(t, "", result.Records[2]["Age"])
	assert.Equal(t, "", result.Records[2]["City"])
}

func TestCSVParser_SupportedFormats(t *testing.T) {
	parser := NewCSVParser(nil)
	formats := parser.SupportedFormats()

	assert.Equal(t, []string{".csv"}, formats)
}

func TestJSONParser_Parse(t *testing.T) {
	tempDir := setupTestFiles(t)
	jsonPath := filepath.Join(tempDir, "test.json")

	parser := NewJSONParser(nil)
	result, err := parser.Parse(context.Background(), jsonPath)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, len(result.Records))
	assert.Equal(t, "JSON", result.Format)

	// Verify first record
	assert.Equal(t, "John Doe", result.Records[0]["Name"])
	assert.Equal(t, float64(30), result.Records[0]["Age"]) // JSON numbers are float64
	assert.Equal(t, "New York", result.Records[0]["City"])
}

func TestJSONParser_ParseStream(t *testing.T) {
	jsonContent := `[
		{"product": "Widget A", "price": 10.99},
		{"product": "Widget B", "price": 20.50}
	]`
	reader := bytes.NewReader([]byte(jsonContent))

	parser := NewJSONParser(nil)
	result, err := parser.ParseStream(context.Background(), reader)

	require.NoError(t, err)
	assert.Equal(t, 2, len(result.Records))
	assert.Equal(t, "JSON", result.Format)
}

func TestJSONParser_SupportedFormats(t *testing.T) {
	parser := NewJSONParser(nil)
	formats := parser.SupportedFormats()

	assert.Equal(t, []string{".json"}, formats)
}

func TestJSONLParser_Parse(t *testing.T) {
	tempDir := setupTestFiles(t)
	jsonlPath := filepath.Join(tempDir, "test.jsonl")

	parser := NewJSONLParser(nil)
	result, err := parser.Parse(context.Background(), jsonlPath)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, len(result.Records))
	assert.Equal(t, "JSONL", result.Format)

	// Verify first record
	assert.Equal(t, "John Doe", result.Records[0]["Name"])
	assert.Equal(t, float64(30), result.Records[0]["Age"])
	assert.Equal(t, "New York", result.Records[0]["City"])
}

func TestJSONLParser_ParseStream(t *testing.T) {
	jsonlContent := `{"product": "Widget A", "price": 10.99}
{"product": "Widget B", "price": 20.50}
{"product": "Widget C", "price": 5.25}
`
	reader := bytes.NewReader([]byte(jsonlContent))

	parser := NewJSONLParser(nil)
	result, err := parser.ParseStream(context.Background(), reader)

	require.NoError(t, err)
	assert.Equal(t, 3, len(result.Records))
	assert.Equal(t, "JSONL", result.Format)
}

func TestJSONLParser_SkipEmptyLines(t *testing.T) {
	jsonlContent := `{"name": "John"}

{"name": "Jane"}

`
	reader := bytes.NewReader([]byte(jsonlContent))

	parser := NewJSONLParser(nil)
	result, err := parser.ParseStream(context.Background(), reader)

	require.NoError(t, err)
	assert.Equal(t, 2, len(result.Records))
	assert.Equal(t, 2, result.SkippedRows) // 2 empty lines skipped
}

func TestJSONLParser_SkipMalformedLines(t *testing.T) {
	jsonlContent := `{"name": "John"}
{invalid json}
{"name": "Jane"}
`
	reader := bytes.NewReader([]byte(jsonlContent))

	parser := NewJSONLParser(nil)
	result, err := parser.ParseStream(context.Background(), reader)

	require.NoError(t, err)
	assert.Equal(t, 2, len(result.Records)) // Only valid lines
	assert.Equal(t, 1, result.SkippedRows)  // 1 malformed line skipped
}

func TestJSONLParser_SupportedFormats(t *testing.T) {
	parser := NewJSONLParser(nil)
	formats := parser.SupportedFormats()

	expected := []string{".jsonl", ".ndjson", ".jsonnl"}
	assert.Equal(t, expected, formats)
}

func TestJSONLParser_AllVariants(t *testing.T) {
	tempDir := setupTestFiles(t)
	parser := NewJSONLParser(nil)

	variants := []string{"test.jsonl", "test.ndjson", "test.jsonnl"}

	for _, filename := range variants {
		t.Run(filename, func(t *testing.T) {
			filePath := filepath.Join(tempDir, filename)
			result, err := parser.Parse(context.Background(), filePath)

			require.NoError(t, err)
			assert.Equal(t, 3, len(result.Records))
			assert.Equal(t, "JSONL", result.Format)
		})
	}
}

func TestParserFactory_GetParser(t *testing.T) {
	factory := NewParserFactory(nil)

	tests := []struct {
		ext      string
		expected string
	}{
		{".csv", "*parsers.CSVParser"},
		{".xlsx", "*parsers.ExcelParser"},
		{".xls", "*parsers.ExcelParser"},
		{".json", "*parsers.JSONParser"},
		{".jsonl", "*parsers.JSONLParser"},
		{".ndjson", "*parsers.JSONLParser"},
		{".jsonnl", "*parsers.JSONLParser"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			parser, err := factory.GetParser(tt.ext)
			require.NoError(t, err)
			assert.NotNil(t, parser)
		})
	}
}

func TestParserFactory_GetParser_Unsupported(t *testing.T) {
	factory := NewParserFactory(nil)

	parser, err := factory.GetParser(".txt")
	assert.Error(t, err)
	assert.Nil(t, parser)
	assert.Contains(t, err.Error(), "no parser found")
}

func TestParserFactory_IsSupported(t *testing.T) {
	factory := NewParserFactory(nil)

	// Supported formats
	assert.True(t, factory.IsSupported(".csv"))
	assert.True(t, factory.IsSupported(".xlsx"))
	assert.True(t, factory.IsSupported(".xls"))
	assert.True(t, factory.IsSupported(".json"))
	assert.True(t, factory.IsSupported(".jsonl"))
	assert.True(t, factory.IsSupported(".ndjson"))
	assert.True(t, factory.IsSupported(".jsonnl"))

	// Unsupported formats
	assert.False(t, factory.IsSupported(".txt"))
	assert.False(t, factory.IsSupported(".pdf"))
	assert.False(t, factory.IsSupported(".xml"))
}

func TestParserFactory_ParseFile(t *testing.T) {
	tempDir := setupTestFiles(t)
	factory := NewParserFactory(nil)

	tests := []struct {
		filename string
		format   string
		records  int
	}{
		{"test.csv", "CSV", 3},
		{"test.json", "JSON", 3},
		{"test.jsonl", "JSONL", 3},
		{"test.ndjson", "JSONL", 3},
		{"test.jsonnl", "JSONL", 3},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			filePath := filepath.Join(tempDir, tt.filename)
			result, err := factory.ParseFile(context.Background(), filePath)

			require.NoError(t, err)
			assert.Equal(t, tt.format, result.Format)
			assert.Equal(t, tt.records, len(result.Records))
		})
	}
}

func TestParserFactory_SupportedFormats(t *testing.T) {
	factory := NewParserFactory(nil)
	formats := factory.SupportedFormats()

	// Should include all formats
	expectedFormats := []string{".csv", ".xlsx", ".xls", ".json", ".jsonl", ".ndjson", ".jsonnl"}

	for _, expected := range expectedFormats {
		assert.Contains(t, formats, expected)
	}
}

func TestParserConfig_MaxFileSize(t *testing.T) {
	tempDir := t.TempDir()

	// Create a CSV file
	content := `Name,Age
John,30
Jane,25
Bob,35
`
	csvPath := filepath.Join(tempDir, "test.csv")
	require.NoError(t, os.WriteFile(csvPath, []byte(content), 0644))

	// Set very small max file size
	config := DefaultParserConfig()
	config.MaxFileSize = 10 // Only 10 bytes

	parser := NewCSVParser(config)
	_, err := parser.Parse(context.Background(), csvPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestContext_Cancellation(t *testing.T) {
	// Create a large dataset
	var buf bytes.Buffer
	buf.WriteString("Name,Age\n")
	for i := 0; i < 10000; i++ {
		buf.WriteString("John,30\n")
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	parser := NewCSVParser(nil)
	_, err := parser.ParseStream(ctx, &buf)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestDefaultParserConfig(t *testing.T) {
	config := DefaultParserConfig()

	assert.Equal(t, 10000, config.MaxRowsInMemory)
	assert.True(t, config.SkipEmptyRows)
	assert.True(t, config.TrimWhitespace)
	assert.Equal(t, int64(500*1024*1024), config.MaxFileSize) // 500 MB
}

func TestParseResult_Structure(t *testing.T) {
	result := &ParseResult{
		Records:     []Record{{"name": "John"}},
		TotalRows:   1,
		SkippedRows: 0,
		Columns:     []string{"name"},
		Format:      "CSV",
	}

	assert.NotNil(t, result.Records)
	assert.Equal(t, 1, result.TotalRows)
	assert.Equal(t, 0, result.SkippedRows)
	assert.Equal(t, []string{"name"}, result.Columns)
	assert.Equal(t, "CSV", result.Format)
}