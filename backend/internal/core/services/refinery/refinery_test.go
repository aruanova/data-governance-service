package refinery

import (
	"strings"
	"testing"
)

// TestRefineryV1Spanish_BasicFunctionality tests the basic cleaning process
func TestRefineryV1Spanish_BasicFunctionality(t *testing.T) {
	// Create V1 Spanish refinery
	refinery := NewRefineryV1Spanish(nil)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "PROMO P1 TV example from FILE_PROCESSING_FLOW.md",
			input:    "PROMO P1 TV 15 SEG (2024)",
			expected: "promo tv seg",
		},
		{
			name:     "TELEVISA S.A. vendor name",
			input:    "TELEVISA S.A.",
			expected: "televisa",
		},
		{
			name:     "MATERIAL POP DISPLAY",
			input:    "MATERIAL POP DISPLAY",
			expected: "material pop display",
		},
		{
			name:     "IMPRESIONES MX vendor",
			input:    "IMPRESIONES MX",
			expected: "impresiones", // MX is removed (2 chars < MIN_LEN=3 and not in TO_KEEP)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := refinery.Process(tt.input)
			if result != tt.expected {
				t.Errorf("Process(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestRefineryV1Spanish_ComplexCases tests complex cleaning scenarios
func TestRefineryV1Spanish_ComplexCases(t *testing.T) {
	refinery := NewRefineryV1Spanish(nil)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Remove period codes (P1, P2, etc)",
			input:    "PROMO P1 TV P2 15 SEG",
			expected: "promo tv seg",
		},
		{
			name:     "Remove Spanish months",
			input:    "EVENTO ENERO FEBRERO MARZO 2024",
			expected: "evento",
		},
		{
			name:     "Preserve business terms",
			input:    "TV GPS MPLS DSL MXN USD",
			expected: "tv gps mpls dsl mxn usd",
		},
		{
			name:     "Remove prefixed codes",
			input:    "PF047-0187 CAFE MEXICO",
			expected: "cafe mexico",
		},
		{
			name:     "Spanish accents normalization",
			input:    "CAFÉ MÉXICO TELEVISIÓN",
			expected: "cafe mexico television",
		},
		{
			name:     "Preserve Ñ character",
			input:    "DISEÑO CAMPAÑA",
			expected: "diseño campaña",
		},
		{
			name:     "Remove SOL patterns",
			input:    "MATERIAL .SOL.JESUS TREVIÑO POP",
			expected: "material", // SOL pattern .* removes everything after it (matches Python behavior)
		},
		{
			name:     "Clean separators",
			input:    "PROMO/TV-RADIO.DIGITAL",
			expected: "promo tv radio digital",
		},
		{
			name:     "Remove short words",
			input:    "EL LA DE UN DOS TV RADIO",
			expected: "dos tv radio", // "DOS" is 3 chars (>= MIN_LEN) and not in TO_REMOVE, so it stays
		},
		{
			name:     "Remove alphanumeric words",
			input:    "PROMO ABC123 TV XYZ456",
			expected: "promo tv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := refinery.Process(tt.input)
			if result != tt.expected {
				t.Errorf("Process(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestRefineryV1Spanish_PreserveBusinessTerms tests that important terms are kept
func TestRefineryV1Spanish_PreserveBusinessTerms(t *testing.T) {
	refinery := NewRefineryV1Spanish(nil)

	businessTerms := []string{
		"SI", "NO", "GPS", "MPLS", "DSL", "MXN", "MXP", "USD",
		"RX", "TC", "TG", "TV", "POP", "MEDIOS",
	}

	for _, term := range businessTerms {
		t.Run("Preserve_"+term, func(t *testing.T) {
			input := "PROMO " + term + " CAMPAÑA"
			result := refinery.Process(input)

			// The term should appear in lowercase in the result
			expectedTerm := strings.ToLower(term)
			if !strings.Contains(result, expectedTerm) {
				t.Errorf("Process(%q) = %q, expected to contain %q", input, result, expectedTerm)
			}
		})
	}
}

// TestRefineryPipeline_BatchProcessing tests the pipeline batch processing
func TestRefineryPipeline_BatchProcessing(t *testing.T) {
	pipeline, err := NewPipeline("v1", nil)
	if err != nil {
		t.Fatalf("Failed to create pipeline: %v", err)
	}

	// Test data from FILE_PROCESSING_FLOW.md example
	inputs := []string{
		"PROMO P1 TV 15 SEG (2024)",
		"TELEVISA S.A.",
		"MATERIAL POP DISPLAY",
		"IMPRESIONES MX",
	}

	expected := []string{
		"promo tv seg",
		"televisa",
		"material pop display",
		"impresiones", // MX removed (respects Python behavior)
	}

	results := pipeline.CleanBatch(inputs)

	if len(results) != len(expected) {
		t.Fatalf("CleanBatch returned %d results, expected %d", len(results), len(expected))
	}

	for i, result := range results {
		if result != expected[i] {
			t.Errorf("CleanBatch[%d] = %q, expected %q", i, result, expected[i])
		}
	}
}

// TestRefineryPipeline_AliasBackwardCompatibility tests alias support
func TestRefineryPipeline_AliasBackwardCompatibility(t *testing.T) {
	// Test that "spanish" alias works (backward compatibility with Python)
	pipeline, err := NewPipeline("spanish", nil)
	if err != nil {
		t.Fatalf("Failed to create pipeline with 'spanish' alias: %v", err)
	}

	if pipeline.GetVersion() != "v1" {
		t.Errorf("Pipeline version = %q, expected 'v1'", pipeline.GetVersion())
	}

	// Test processing
	input := "PROMO P1 TV 15 SEG (2024)"
	expected := "promo tv seg"
	result := pipeline.CleanText(input)

	if result != expected {
		t.Errorf("CleanText(%q) = %q, expected %q", input, result, expected)
	}
}

// TestRefineryRegistry tests the registry functionality
func TestRefineryRegistry(t *testing.T) {
	// Test listing available refineries
	available := ListAvailable()
	if len(available) == 0 {
		t.Error("ListAvailable returned no refineries")
	}

	found := false
	for _, version := range available {
		if version == "v1" {
			found = true
			break
		}
	}

	if !found {
		t.Error("v1 refinery not found in available list")
	}

	// Test metadata
	metadata := ListAvailableWithMetadata()
	v1Meta, exists := metadata["v1"]
	if !exists {
		t.Error("v1 metadata not found")
	}

	if v1Meta["name"] != "Spanish Text Cleaning" {
		t.Errorf("v1 name = %q, expected 'Spanish Text Cleaning'", v1Meta["name"])
	}
}

// TestRefineryV1Spanish_EmptyAndNullHandling tests edge cases
func TestRefineryV1Spanish_EmptyAndNullHandling(t *testing.T) {
	refinery := NewRefineryV1Spanish(nil)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Only whitespace",
			input:    "   ",
			expected: "",
		},
		{
			name:     "Only special characters",
			input:    ".,;:!?",
			expected: "",
		},
		{
			name:     "Only numbers",
			input:    "123 456 789",
			expected: "",
		},
		{
			name:     "Mixed valid and invalid",
			input:    "TV 123 RADIO 456",
			expected: "tv radio",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := refinery.Process(tt.input)
			if result != tt.expected {
				t.Errorf("Process(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// BenchmarkRefineryV1Spanish_SingleText benchmarks single text processing
func BenchmarkRefineryV1Spanish_SingleText(b *testing.B) {
	refinery := NewRefineryV1Spanish(nil)
	input := "PROMO P1 TV 15 SEG (2024) CAFÉ MÉXICO"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = refinery.Process(input)
	}
}

// BenchmarkRefineryPipeline_Batch benchmarks batch processing
func BenchmarkRefineryPipeline_Batch(b *testing.B) {
	pipeline, _ := NewPipeline("v1", nil)

	// Create 100 sample texts
	inputs := make([]string, 100)
	for i := 0; i < 100; i++ {
		inputs[i] = "PROMO P1 TV 15 SEG (2024) CAFÉ MÉXICO"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pipeline.CleanBatch(inputs)
	}
}