package refinery

import (
	"fmt"
	"sync"
)

// RefineryFactory is a function type that creates a refinery instance
type RefineryFactory func(config map[string]interface{}) BaseRefinery

// Registry manages all available refinery implementations
type Registry struct {
	mu         sync.RWMutex
	refineries map[string]RefineryFactory
	aliases    map[string]string // For backward compatibility
}

// Global registry instance
var globalRegistry = &Registry{
	refineries: make(map[string]RefineryFactory),
	aliases:    make(map[string]string),
}

// Register adds a refinery to the registry with optional aliases
func Register(version string, factory RefineryFactory, aliases ...string) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	globalRegistry.refineries[version] = factory

	// Register aliases
	for _, alias := range aliases {
		globalRegistry.aliases[alias] = version
	}
}

// Get retrieves a refinery factory by version or alias
func Get(identifier string) (RefineryFactory, error) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	// Check if it's an alias first
	if version, exists := globalRegistry.aliases[identifier]; exists {
		identifier = version
	}

	factory, exists := globalRegistry.refineries[identifier]
	if !exists {
		return nil, fmt.Errorf("refinery '%s' not found. Available: %v", identifier, ListAvailable())
	}

	return factory, nil
}

// Create creates a new refinery instance
func Create(identifier string, config map[string]interface{}) (BaseRefinery, error) {
	factory, err := Get(identifier)
	if err != nil {
		return nil, err
	}

	return factory(config), nil
}

// ListAvailable returns a list of all available refinery versions
func ListAvailable() []string {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	versions := make([]string, 0, len(globalRegistry.refineries))
	for version := range globalRegistry.refineries {
		versions = append(versions, version)
	}

	return versions
}

// ListAvailableWithMetadata returns detailed information about all refineries
func ListAvailableWithMetadata() map[string]map[string]interface{} {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	result := make(map[string]map[string]interface{})

	for version, factory := range globalRegistry.refineries {
		// Create a temporary instance to get metadata
		instance := factory(nil)

		// Get aliases for this version
		var versionAliases []string
		for alias, v := range globalRegistry.aliases {
			if v == version {
				versionAliases = append(versionAliases, alias)
			}
		}

		result[version] = map[string]interface{}{
			"name":        instance.GetName(),
			"description": instance.GetDescription(),
			"aliases":     versionAliases,
			"steps":       instance.GetPipelineSteps(),
		}
	}

	return result
}

// init registers the default refineries
func init() {
	// Register V1 Spanish (based on proven Python V3)
	Register("v1", func(config map[string]interface{}) BaseRefinery {
		return NewRefineryV1Spanish(config)
	}, "spanish", "v1-spanish", "standard")

	// Future: Register V2, V3, etc. as they are developed
	// Register("v2", NewRefineryV2Factory, "english", "v2-english")
}