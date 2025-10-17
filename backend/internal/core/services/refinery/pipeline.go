package refinery

import (
	"fmt"
)

// Pipeline orchestrates the text cleaning process using a specific refinery
type Pipeline struct {
	refinery BaseRefinery
	version  string
}

// NewPipeline creates a new refinery pipeline
// refineryType can be a version (e.g., "v1") or an alias (e.g., "spanish")
func NewPipeline(refineryType string, customConfig map[string]interface{}) (*Pipeline, error) {
	// Handle backward compatibility with Python system
	// "spanish" -> "v1" (our proven V3 from Python)
	// Future: "english" -> "v2" when implemented

	refinery, err := Create(refineryType, customConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create refinery: %w", err)
	}

	return &Pipeline{
		refinery: refinery,
		version:  refinery.GetVersion(),
	}, nil
}

// CleanText processes a single text string
func (p *Pipeline) CleanText(text string) string {
	return p.refinery.Process(text)
}

// CleanBatch processes a batch of texts
func (p *Pipeline) CleanBatch(texts []string) []string {
	results := make([]string, len(texts))
	for i, text := range texts {
		results[i] = p.refinery.Process(text)
	}
	return results
}

// GetVersion returns the refinery version being used
func (p *Pipeline) GetVersion() string {
	return p.version
}

// GetName returns the refinery name
func (p *Pipeline) GetName() string {
	return p.refinery.GetName()
}

// GetDescription returns the refinery description
func (p *Pipeline) GetDescription() string {
	return p.refinery.GetDescription()
}

// GetPipelineSteps returns the processing steps
func (p *Pipeline) GetPipelineSteps() []string {
	return p.refinery.GetPipelineSteps()
}