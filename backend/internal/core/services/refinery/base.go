package refinery

// BaseRefinery defines the interface that all refinery implementations must follow
// This enables a plugin architecture where different cleaning strategies can be swapped
type BaseRefinery interface {
	// Process cleans a single text string through the refinery pipeline
	Process(text string) string

	// GetVersion returns the version identifier (e.g., "v1", "v2", "v3")
	GetVersion() string

	// GetName returns a human-readable name
	GetName() string

	// GetDescription returns what this refinery does
	GetDescription() string

	// GetDefaultConfig returns the default configuration
	GetDefaultConfig() map[string]interface{}

	// GetPipelineSteps returns the list of processing steps in order
	GetPipelineSteps() []string
}

// ProcessingStep represents a single text transformation function
type ProcessingStep func(string) string

// RefineryConfig holds configuration for a refinery
type RefineryConfig struct {
	// Character and word filters
	AllowedChars string   `json:"allowed_chars"`
	ToKeep       []string `json:"to_keep"`
	ToRemove     []string `json:"to_remove"`
	MinLen       int      `json:"min_len"`
	SepChars     string   `json:"sep_chars"`
	Vowels       string   `json:"vowels"`

	// Processing flags
	FixMojibakeEncoding          bool `json:"fix_mojibake_encoding"`
	RemoveAdvancedPrefixedCodes  bool `json:"remove_advanced_prefixed_codes"`
	NormalizeSpanishAccents      bool `json:"normalize_spanish_accents"`
	RemovePeriodCodes            bool `json:"remove_period_codes"`
	MakeUppercase                bool `json:"make_uppercase"`
	MakeLowercase                bool `json:"make_lowercase"`
	RemoveTrailingSolicitante    bool `json:"remove_trailing_solicitante"`
	ReplaceSeparatorsWithSpaces  bool `json:"replace_separators_with_spaces"`
	RemoveMultipleWhitespace     bool `json:"remove_multiple_whitespace"`
	RemoveSpecialChars           bool `json:"remove_special_chars"`
	RemoveWordsFromList          bool `json:"remove_words_from_list"`
	RemoveAlphanumericWords      bool `json:"remove_alphanumeric_words"`
	RemoveAllNumbersWordsExcept  bool `json:"remove_all_numbers_words_except"`
	RemoveWordsByMinLen          bool `json:"remove_words_by_min_len"`
	RemoveAllConsonantsWords     bool `json:"remove_all_consonants_words"`

	// Additional settings
	SeparatorReplacement string `json:"separator_replacement"`
}