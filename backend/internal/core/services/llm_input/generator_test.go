package llm_input

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerator_GenerateInput(t *testing.T) {
	generator := NewGenerator(nil)

	records := []Record{
		{
			RowIndex: 0,
			OriginalData: map[string]interface{}{
				"LineDescription": "PROMO TV 15 SEG",
			},
			CleanedData: map[string]interface{}{
				"cleanLineDescription": "promo tv seg",
				"cleanAccount":         "5000",
			},
		},
		{
			RowIndex: 1,
			OriginalData: map[string]interface{}{
				"LineDescription": "REVISTA DIGITAL",
			},
			CleanedData: map[string]interface{}{
				"cleanLineDescription": "revista digital",
				"cleanAccount":         "6000",
			},
		},
	}

	config := DefaultGeneratorConfig()
	input, err := generator.GenerateInput(records, config)

	require.NoError(t, err)
	require.NotNil(t, input)

	assert.Equal(t, 2, input.Stats.TotalRecords)
	assert.Equal(t, 2, len(input.Records))
	assert.Greater(t, input.Stats.EstimatedTokens, 0)

	// Verify row indices are preserved
	assert.Equal(t, 0, input.Records[0].RowIndex)
	assert.Equal(t, 1, input.Records[1].RowIndex)

	// Verify clean fields are included
	assert.Contains(t, input.Records[0].Data, "cleanLineDescription")
	assert.Contains(t, input.Records[0].Data, "cleanAccount")
}

func TestGenerator_DetectCleanFields(t *testing.T) {
	generator := NewGenerator(nil)

	record := Record{
		CleanedData: map[string]interface{}{
			"cleanLineDescription": "test",
			"cleanAccount":         "5000",
			"regularField":         "value",
		},
	}

	fields := generator.DetectCleanFields(record)

	assert.Len(t, fields, 2)
	assert.Contains(t, fields, "cleanLineDescription")
	assert.Contains(t, fields, "cleanAccount")
	assert.NotContains(t, fields, "regularField")
}

func TestGenerator_DetectCleanFields_CaseInsensitive(t *testing.T) {
	generator := NewGenerator(nil)

	record := Record{
		CleanedData: map[string]interface{}{
			"CleanLineDescription": "test",  // Capital C
			"CLEANACCOUNT":         "5000",  // All caps
			"cleanBalance":         "1000",  // lowercase
		},
	}

	fields := generator.DetectCleanFields(record)

	assert.Len(t, fields, 3)
	assert.Contains(t, fields, "CleanLineDescription")
	assert.Contains(t, fields, "CLEANACCOUNT")
	assert.Contains(t, fields, "cleanBalance")
}

func TestGenerator_DetectCleanFields_FallbackToOriginal(t *testing.T) {
	generator := NewGenerator(nil)

	record := Record{
		OriginalData: map[string]interface{}{
			"cleanLineDescription": "test",
		},
		CleanedData: map[string]interface{}{
			"otherField": "value",
		},
	}

	fields := generator.DetectCleanFields(record)

	// Should fallback to OriginalData when CleanedData has no clean fields
	assert.Len(t, fields, 1)
	assert.Contains(t, fields, "cleanLineDescription")
}

func TestGenerator_EstimateTokenCount(t *testing.T) {
	generator := NewGenerator(nil)

	input := &LLMInput{
		Records: []CleanRecord{
			{
				RowIndex: 0,
				Data: map[string]interface{}{
					"cleanLineDescription": "promo tv seg",
				},
			},
		},
	}

	tokens := generator.EstimateTokenCount(input)

	// Should be > 0 and include prompt overhead (~300 tokens)
	assert.Greater(t, tokens, 300)
}

func TestGenerator_GenerateChunks(t *testing.T) {
	generator := NewGenerator(nil)

	// Create 250 records
	records := make([]Record, 250)
	for i := 0; i < 250; i++ {
		records[i] = Record{
			RowIndex: i,
			CleanedData: map[string]interface{}{
				"cleanLineDescription": "test",
			},
		}
	}

	config := DefaultGeneratorConfig().WithChunkSize(100)
	chunks, err := generator.GenerateChunks(records, config)

	require.NoError(t, err)
	assert.Len(t, chunks, 3) // 250 / 100 = 3 chunks

	// First chunk should have 100 records
	assert.Equal(t, 100, len(chunks[0].Records))

	// Second chunk should have 100 records
	assert.Equal(t, 100, len(chunks[1].Records))

	// Third chunk should have 50 records
	assert.Equal(t, 50, len(chunks[2].Records))

	// Verify chunk metadata
	assert.Equal(t, 1, chunks[0].Metadata.ChunkNumber)
	assert.Equal(t, 3, chunks[0].Metadata.TotalChunks)

	assert.Equal(t, 2, chunks[1].Metadata.ChunkNumber)
	assert.Equal(t, 3, chunks[1].Metadata.TotalChunks)

	assert.Equal(t, 3, chunks[2].Metadata.ChunkNumber)
	assert.Equal(t, 3, chunks[2].Metadata.TotalChunks)
}

func TestGenerator_GenerateChunks_ExactDivision(t *testing.T) {
	generator := NewGenerator(nil)

	// Create exactly 200 records
	records := make([]Record, 200)
	for i := 0; i < 200; i++ {
		records[i] = Record{
			RowIndex: i,
			CleanedData: map[string]interface{}{
				"cleanLineDescription": "test",
			},
		}
	}

	config := DefaultGeneratorConfig().WithChunkSize(100)
	chunks, err := generator.GenerateChunks(records, config)

	require.NoError(t, err)
	assert.Len(t, chunks, 2) // Exactly 2 chunks

	assert.Equal(t, 100, len(chunks[0].Records))
	assert.Equal(t, 100, len(chunks[1].Records))
}

func TestGenerator_ToJSON_Compact(t *testing.T) {
	generator := NewGenerator(nil)

	input := &LLMInput{
		Records: []CleanRecord{
			{
				RowIndex: 0,
				Data: map[string]interface{}{
					"cleanLineDescription": "test",
				},
			},
		},
	}

	jsonBytes, err := generator.ToJSON(input, true)
	require.NoError(t, err)

	// Compact JSON should not have newlines (except in strings)
	jsonStr := string(jsonBytes)
	assert.NotContains(t, jsonStr, "\n  ")
}

func TestGenerator_ToJSON_Pretty(t *testing.T) {
	generator := NewGenerator(nil)

	input := &LLMInput{
		Records: []CleanRecord{
			{
				RowIndex: 0,
				Data: map[string]interface{}{
					"cleanLineDescription": "test",
				},
			},
		},
	}

	jsonBytes, err := generator.ToJSON(input, false)
	require.NoError(t, err)

	// Pretty JSON should have indentation
	jsonStr := string(jsonBytes)
	assert.Contains(t, jsonStr, "\n  ")
}

func TestGenerator_ValidateInput_Success(t *testing.T) {
	generator := NewGenerator(nil)

	input := &LLMInput{
		Metadata: InputMetadata{
			Fields: []string{"cleanLineDescription"},
		},
		Records: []CleanRecord{
			{
				RowIndex: 0,
				Data: map[string]interface{}{
					"cleanLineDescription": "test",
				},
			},
		},
	}

	err := generator.ValidateInput(input)
	assert.NoError(t, err)
}

func TestGenerator_ValidateInput_NilInput(t *testing.T) {
	generator := NewGenerator(nil)

	err := generator.ValidateInput(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input is nil")
}

func TestGenerator_ValidateInput_NoRecords(t *testing.T) {
	generator := NewGenerator(nil)

	input := &LLMInput{
		Metadata: InputMetadata{
			Fields: []string{"cleanLineDescription"},
		},
		Records: []CleanRecord{},
	}

	err := generator.ValidateInput(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no records")
}

func TestGenerator_ValidateInput_NoFields(t *testing.T) {
	generator := NewGenerator(nil)

	input := &LLMInput{
		Metadata: InputMetadata{
			Fields: []string{},
		},
		Records: []CleanRecord{
			{
				RowIndex: 0,
				Data:     map[string]interface{}{"test": "value"},
			},
		},
	}

	err := generator.ValidateInput(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no fields")
}

func TestGenerator_ValidateInput_DuplicateRowIndex(t *testing.T) {
	generator := NewGenerator(nil)

	input := &LLMInput{
		Metadata: InputMetadata{
			Fields: []string{"cleanLineDescription"},
		},
		Records: []CleanRecord{
			{
				RowIndex: 0,
				Data:     map[string]interface{}{"test": "value"},
			},
			{
				RowIndex: 0, // Duplicate!
				Data:     map[string]interface{}{"test": "value2"},
			},
		},
	}

	err := generator.ValidateInput(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate row_index")
}

func TestGenerator_ValidateInput_EmptyData(t *testing.T) {
	generator := NewGenerator(nil)

	input := &LLMInput{
		Metadata: InputMetadata{
			Fields: []string{"cleanLineDescription"},
		},
		Records: []CleanRecord{
			{
				RowIndex: 0,
				Data:     map[string]interface{}{}, // Empty!
			},
		},
	}

	err := generator.ValidateInput(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has no data")
}

func TestBuildRecordFromMap(t *testing.T) {
	original := map[string]interface{}{
		"LineDescription": "PROMO TV",
	}

	cleaned := map[string]interface{}{
		"cleanLineDescription": "promo tv",
	}

	record := BuildRecordFromMap(5, original, cleaned)

	assert.Equal(t, 5, record.RowIndex)
	assert.Equal(t, original, record.OriginalData)
	assert.Equal(t, cleaned, record.CleanedData)
}

func TestExtractCleanFields(t *testing.T) {
	data := map[string]interface{}{
		"cleanLineDescription": "promo tv",
		"cleanAccount":         "5000",
		"LineDescription":      "PROMO TV",
		"Account":              "5000",
	}

	clean := ExtractCleanFields(data)

	assert.Len(t, clean, 2)
	assert.Contains(t, clean, "cleanLineDescription")
	assert.Contains(t, clean, "cleanAccount")
	assert.NotContains(t, clean, "LineDescription")
	assert.NotContains(t, clean, "Account")
}

func TestGenerator_GenerateInput_EmptyRecords(t *testing.T) {
	generator := NewGenerator(nil)

	records := []Record{}
	config := DefaultGeneratorConfig()

	input, err := generator.GenerateInput(records, config)

	assert.Error(t, err)
	assert.Nil(t, input)
	assert.Contains(t, err.Error(), "no records")
}

func TestGenerator_GenerateInput_NoCleanFields(t *testing.T) {
	generator := NewGenerator(nil)

	records := []Record{
		{
			RowIndex: 0,
			CleanedData: map[string]interface{}{
				"regularField": "value",
			},
		},
	}

	config := DefaultGeneratorConfig()
	input, err := generator.GenerateInput(records, config)

	assert.Error(t, err)
	assert.Nil(t, input)
	assert.Contains(t, err.Error(), "no clean fields")
}

func TestGenerator_GenerateInput_CustomFields(t *testing.T) {
	generator := NewGenerator(nil)

	records := []Record{
		{
			RowIndex: 0,
			CleanedData: map[string]interface{}{
				"cleanLineDescription": "test",
				"cleanAccount":         "5000",
				"cleanBalance":         "1000",
			},
		},
	}

	// Only include specific fields
	config := DefaultGeneratorConfig().WithFields([]string{"cleanLineDescription", "cleanBalance"})
	input, err := generator.GenerateInput(records, config)

	require.NoError(t, err)

	// Should only have the 2 specified fields
	assert.Len(t, input.Metadata.Fields, 2)
	assert.Contains(t, input.Records[0].Data, "cleanLineDescription")
	assert.Contains(t, input.Records[0].Data, "cleanBalance")
	assert.NotContains(t, input.Records[0].Data, "cleanAccount")
}

func TestGenerator_JSONSerializationRoundTrip(t *testing.T) {
	generator := NewGenerator(nil)

	records := []Record{
		{
			RowIndex: 0,
			CleanedData: map[string]interface{}{
				"cleanLineDescription": "promo tv",
			},
		},
	}

	config := DefaultGeneratorConfig()
	input, err := generator.GenerateInput(records, config)
	require.NoError(t, err)

	// Serialize to JSON
	jsonBytes, err := generator.ToJSON(input, true)
	require.NoError(t, err)

	// Deserialize back
	var decoded LLMInput
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)

	// Verify data integrity
	assert.Equal(t, input.Stats.TotalRecords, decoded.Stats.TotalRecords)
	assert.Equal(t, input.Records[0].RowIndex, decoded.Records[0].RowIndex)
	assert.Equal(t, input.Records[0].Data["cleanLineDescription"], decoded.Records[0].Data["cleanLineDescription"])
}

func BenchmarkGenerator_GenerateInput(b *testing.B) {
	generator := NewGenerator(nil)

	// Create 1000 records
	records := make([]Record, 1000)
	for i := 0; i < 1000; i++ {
		records[i] = Record{
			RowIndex: i,
			CleanedData: map[string]interface{}{
				"cleanLineDescription": "promo tv seg",
				"cleanAccount":         "5000",
			},
		}
	}

	config := DefaultGeneratorConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = generator.GenerateInput(records, config)
	}
}

func BenchmarkGenerator_GenerateChunks(b *testing.B) {
	generator := NewGenerator(nil)

	// Create 10000 records
	records := make([]Record, 10000)
	for i := 0; i < 10000; i++ {
		records[i] = Record{
			RowIndex: i,
			CleanedData: map[string]interface{}{
				"cleanLineDescription": "promo tv seg",
				"cleanAccount":         "5000",
			},
		}
	}

	config := DefaultGeneratorConfig().WithChunkSize(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = generator.GenerateChunks(records, config)
	}
}