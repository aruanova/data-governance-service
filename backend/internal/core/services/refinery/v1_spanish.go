package refinery

// RefineryV1Spanish implements Version 1 Refinery for the new Go service
// This is based on the proven V3 Enhanced Spanish from the Python system
//
// Features:
// - Mojibake encoding fixes (UTF-8 misinterpreted as Latin-1)
// - Improved pattern matching that preserves important words
// - Spanish accent normalization (preserves ñ)
// - Better SOL. pattern removal
// - Expanded list of preserved business terms
type RefineryV1Spanish struct {
	config   *RefineryConfig
	nodes    *ProcessingNodes
	pipeline []ProcessingStep
}

// NewRefineryV1Spanish creates a new V1 refinery instance
func NewRefineryV1Spanish(customConfig map[string]interface{}) *RefineryV1Spanish {
	// Default configuration for V1 (based on Python V3)
	config := &RefineryConfig{
		AllowedChars: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyzÑñ0123456789 ",
		ToKeep: []string{
			"SI", "NO", "GPS", "MPLS", "DSL", "MXN", "MXP", "USD", "RX", "TC", "TG",
			"TV", "POP", "MEDIOS", "36ROJBRINDIS",
		},
		ToRemove: []string{
			"ENERO", "FEBRERO", "MARZO", "ABRIL", "MAYO", "JUNIO",
			"JULIO", "AGOSTO", "SEPTIEMBRE", "OCTUBRE", "NOVIEMBRE", "DICIEMBRE",
			"ENE", "FEB", "MAR", "ABR", "MAY", "JUN",
			"JUL", "AGO", "SEP", "OCT", "NOV", "DIC",
			"DE", "DEL",
		},
		MinLen:               3,
		SepChars:             ".,-/+&|",
		SeparatorReplacement: " ",
		Vowels:               "AEIOUaeiouYy",

		// Processing flags
		FixMojibakeEncoding:          true,
		RemoveAdvancedPrefixedCodes:  true,
		NormalizeSpanishAccents:      true,
		RemovePeriodCodes:            true,
		MakeUppercase:                true,
		MakeLowercase:                true,
		RemoveTrailingSolicitante:    true,
		ReplaceSeparatorsWithSpaces:  true,
		RemoveMultipleWhitespace:     true,
		RemoveSpecialChars:           true,
		RemoveWordsFromList:          true,
		RemoveAlphanumericWords:      true,
		RemoveAllNumbersWordsExcept:  true,
		RemoveWordsByMinLen:          true,
		RemoveAllConsonantsWords:     true,
	}

	// Apply custom config overrides if provided
	if customConfig != nil {
		applyCustomConfig(config, customConfig)
	}

	// Create processing nodes
	nodes := NewProcessingNodes(config)

	// Build default pipeline
	pipeline := []ProcessingStep{
		nodes.FixMojibakeEncoding,
		nodes.RemoveAdvancedPrefixedCodes,
		nodes.NormalizeSpanishAccents,
		nodes.MakeUppercase,
		nodes.RemoveTrailingSolicitante,
		nodes.ReplaceSeparators,
		nodes.RemoveMultipleWhitespace,
		nodes.RemoveSpecialChars,
		nodes.RemoveWordsFromList,
		nodes.RemovePeriodCodes,
		nodes.RemoveAlphanumericWords,
		nodes.RemoveAllNumbersWordsExcept,
		nodes.RemoveWordsByMinLen,
		nodes.RemoveAllConsonantsWords,
		nodes.MakeLowercase,
	}

	return &RefineryV1Spanish{
		config:   config,
		nodes:    nodes,
		pipeline: pipeline,
	}
}

// Process processes text through the configured pipeline
func (r *RefineryV1Spanish) Process(text string) string {
	for _, step := range r.pipeline {
		text = step(text)
	}
	return text
}

// GetVersion returns the version identifier
func (r *RefineryV1Spanish) GetVersion() string {
	return "v1"
}

// GetName returns the human-readable name
func (r *RefineryV1Spanish) GetName() string {
	return "Spanish Text Cleaning"
}

// GetDescription returns what this refinery does
func (r *RefineryV1Spanish) GetDescription() string {
	return "Spanish text cleaning with mojibake fixes, improved patterns, and business term preservation (based on proven Python V3)"
}

// GetDefaultConfig returns the default configuration
func (r *RefineryV1Spanish) GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"allowed_chars": "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyzÑñ0123456789 ",
		"to_keep": []string{
			"SI", "NO", "GPS", "MPLS", "DSL", "MXN", "MXP", "USD", "RX", "TC", "TG",
			"TV", "POP", "MEDIOS", "36ROJBRINDIS",
		},
		"to_remove": []string{
			"ENERO", "FEBRERO", "MARZO", "ABRIL", "MAYO", "JUNIO",
			"JULIO", "AGOSTO", "SEPTIEMBRE", "OCTUBRE", "NOVIEMBRE", "DICIEMBRE",
			"ENE", "FEB", "MAR", "ABR", "MAY", "JUN",
			"JUL", "AGO", "SEP", "OCT", "NOV", "DIC",
			"DE", "DEL",
		},
		"min_len":               3,
		"sep_chars":             ".,-/+&|",
		"separator_replacement": " ",
		"vowels":                "AEIOUaeiouYy",
		"fix_mojibake_encoding": true,
		"remove_advanced_prefixed_codes": true,
		"normalize_spanish_accents": true,
		"remove_period_codes": true,
		"make_uppercase": true,
		"make_lowercase": true,
		"remove_trailing_solicitante": true,
		"replace_separators_with_spaces": true,
		"remove_multiple_whitespace": true,
		"remove_special_chars": true,
		"remove_words_from_list": true,
		"remove_alphanumeric_words": true,
		"remove_all_numbers_words_except": true,
		"remove_words_by_min_len": true,
		"remove_all_consonants_words": true,
	}
}

// GetPipelineSteps returns the list of processing steps
func (r *RefineryV1Spanish) GetPipelineSteps() []string {
	return []string{
		"fix_mojibake_encoding",
		"remove_advanced_prefixed_codes",
		"normalize_spanish_accents",
		"make_uppercase",
		"remove_trailing_solicitante",
		"replace_separators",
		"remove_multiple_whitespace",
		"remove_special_chars",
		"remove_words_from_list",
		"remove_period_codes",
		"remove_alphanumeric_words",
		"remove_all_numbers_words_except",
		"remove_words_by_min_len",
		"remove_all_consonants_words",
		"make_lowercase",
	}
}

// AddNode adds a processing node to the pipeline at the specified position
func (r *RefineryV1Spanish) AddNode(node ProcessingStep, position int) {
	if position < 0 || position >= len(r.pipeline) {
		r.pipeline = append(r.pipeline, node)
	} else {
		// Insert at position
		r.pipeline = append(r.pipeline[:position+1], r.pipeline[position:]...)
		r.pipeline[position] = node
	}
}

// RemoveNodeAtPosition removes a processing node from the pipeline by position
func (r *RefineryV1Spanish) RemoveNodeAtPosition(position int) {
	if position >= 0 && position < len(r.pipeline) {
		r.pipeline = append(r.pipeline[:position], r.pipeline[position+1:]...)
	}
}

// Helper function to apply custom configuration
func applyCustomConfig(config *RefineryConfig, custom map[string]interface{}) {
	if v, ok := custom["allowed_chars"].(string); ok {
		config.AllowedChars = v
	}
	if v, ok := custom["to_keep"].([]string); ok {
		config.ToKeep = v
	}
	if v, ok := custom["to_remove"].([]string); ok {
		config.ToRemove = v
	}
	if v, ok := custom["min_len"].(int); ok {
		config.MinLen = v
	}
	if v, ok := custom["sep_chars"].(string); ok {
		config.SepChars = v
	}
	if v, ok := custom["vowels"].(string); ok {
		config.Vowels = v
	}
	if v, ok := custom["separator_replacement"].(string); ok {
		config.SeparatorReplacement = v
	}

	// Apply boolean flags
	if v, ok := custom["fix_mojibake_encoding"].(bool); ok {
		config.FixMojibakeEncoding = v
	}
	if v, ok := custom["remove_advanced_prefixed_codes"].(bool); ok {
		config.RemoveAdvancedPrefixedCodes = v
	}
	if v, ok := custom["normalize_spanish_accents"].(bool); ok {
		config.NormalizeSpanishAccents = v
	}
	if v, ok := custom["remove_period_codes"].(bool); ok {
		config.RemovePeriodCodes = v
	}
	if v, ok := custom["make_uppercase"].(bool); ok {
		config.MakeUppercase = v
	}
	if v, ok := custom["make_lowercase"].(bool); ok {
		config.MakeLowercase = v
	}
	if v, ok := custom["remove_trailing_solicitante"].(bool); ok {
		config.RemoveTrailingSolicitante = v
	}
	if v, ok := custom["replace_separators_with_spaces"].(bool); ok {
		config.ReplaceSeparatorsWithSpaces = v
	}
	if v, ok := custom["remove_multiple_whitespace"].(bool); ok {
		config.RemoveMultipleWhitespace = v
	}
	if v, ok := custom["remove_special_chars"].(bool); ok {
		config.RemoveSpecialChars = v
	}
	if v, ok := custom["remove_words_from_list"].(bool); ok {
		config.RemoveWordsFromList = v
	}
	if v, ok := custom["remove_alphanumeric_words"].(bool); ok {
		config.RemoveAlphanumericWords = v
	}
	if v, ok := custom["remove_all_numbers_words_except"].(bool); ok {
		config.RemoveAllNumbersWordsExcept = v
	}
	if v, ok := custom["remove_words_by_min_len"].(bool); ok {
		config.RemoveWordsByMinLen = v
	}
	if v, ok := custom["remove_all_consonants_words"].(bool); ok {
		config.RemoveAllConsonantsWords = v
	}
}