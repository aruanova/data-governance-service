package deduplication

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHashRepository implements HashRepository for testing
type mockHashRepository struct {
	existingHashes map[string]bool
	savedHashes    map[uuid.UUID][]HashEntry
}

func newMockHashRepository() *mockHashRepository {
	return &mockHashRepository{
		existingHashes: make(map[string]bool),
		savedHashes:    make(map[uuid.UUID][]HashEntry),
	}
}

func (m *mockHashRepository) CheckHashExists(ctx context.Context, hash string) (bool, error) {
	return m.existingHashes[hash], nil
}

func (m *mockHashRepository) SaveHashes(ctx context.Context, batchID uuid.UUID, hashes []HashEntry) error {
	m.savedHashes[batchID] = hashes
	// Add kept hashes to existing
	for _, h := range hashes {
		if h.Kept {
			m.existingHashes[h.Hash] = true
		}
	}
	return nil
}

func (m *mockHashRepository) GetBatchHashes(ctx context.Context, batchID uuid.UUID) ([]HashEntry, error) {
	return m.savedHashes[batchID], nil
}

func TestService_DeduplicateLevel1_ExactMatch(t *testing.T) {
	config := Config{
		Strategy:       StrategyExact,
		CleanFields:    []string{"cleanLineDescription"},
		EnableLevel2:   false,
		StoreHashes:    false,
		CaseSensitive:  false,
		TrimWhitespace: true,
	}

	service := NewService(config, nil, nil)

	records := []Record{
		{RowIndex: 0, Data: map[string]interface{}{"cleanLineDescription": "promo tv"}},
		{RowIndex: 1, Data: map[string]interface{}{"cleanLineDescription": "promo tv"}}, // Duplicate
		{RowIndex: 2, Data: map[string]interface{}{"cleanLineDescription": "revista digital"}},
		{RowIndex: 3, Data: map[string]interface{}{"cleanLineDescription": "promo tv"}}, // Duplicate
		{RowIndex: 4, Data: map[string]interface{}{"cleanLineDescription": "libro mental"}},
	}

	batchID := uuid.New()
	result, err := service.Deduplicate(context.Background(), batchID, records)

	require.NoError(t, err)
	assert.Equal(t, 5, result.OriginalCount)
	assert.Equal(t, 3, result.DeduplicatedCount)
	assert.Equal(t, 2, result.RemovedCount)
	assert.Equal(t, 2, result.Stats.Level1Duplicates)
	assert.Equal(t, 0, result.Stats.Level2Duplicates)
	assert.Len(t, result.Records, 3)

	// Verify unique values remain
	descriptions := make(map[string]bool)
	for _, record := range result.Records {
		desc := record.Data["cleanLineDescription"].(string)
		descriptions[desc] = true
	}

	assert.True(t, descriptions["promo tv"])
	assert.True(t, descriptions["revista digital"])
	assert.True(t, descriptions["libro mental"])
}

func TestService_DeduplicateLevel1_CaseSensitive(t *testing.T) {
	config := Config{
		Strategy:       StrategyExact,
		CleanFields:    []string{"cleanLineDescription"},
		EnableLevel2:   false,
		StoreHashes:    false,
		CaseSensitive:  true, // Case sensitive
		TrimWhitespace: true,
	}

	service := NewService(config, nil, nil)

	records := []Record{
		{RowIndex: 0, Data: map[string]interface{}{"cleanLineDescription": "PROMO TV"}},
		{RowIndex: 1, Data: map[string]interface{}{"cleanLineDescription": "promo tv"}}, // Different case
		{RowIndex: 2, Data: map[string]interface{}{"cleanLineDescription": "Promo Tv"}}, // Different case
	}

	batchID := uuid.New()
	result, err := service.Deduplicate(context.Background(), batchID, records)

	require.NoError(t, err)
	assert.Equal(t, 3, result.OriginalCount)
	assert.Equal(t, 3, result.DeduplicatedCount) // All unique because case-sensitive
	assert.Equal(t, 0, result.RemovedCount)
}

func TestService_DeduplicateLevel1_CaseInsensitive(t *testing.T) {
	config := Config{
		Strategy:       StrategyExact,
		CleanFields:    []string{"cleanLineDescription"},
		EnableLevel2:   false,
		StoreHashes:    false,
		CaseSensitive:  false, // Case insensitive
		TrimWhitespace: true,
	}

	service := NewService(config, nil, nil)

	records := []Record{
		{RowIndex: 0, Data: map[string]interface{}{"cleanLineDescription": "PROMO TV"}},
		{RowIndex: 1, Data: map[string]interface{}{"cleanLineDescription": "promo tv"}}, // Same (case-insensitive)
		{RowIndex: 2, Data: map[string]interface{}{"cleanLineDescription": "Promo Tv"}}, // Same (case-insensitive)
	}

	batchID := uuid.New()
	result, err := service.Deduplicate(context.Background(), batchID, records)

	require.NoError(t, err)
	assert.Equal(t, 3, result.OriginalCount)
	assert.Equal(t, 1, result.DeduplicatedCount) // All same when case-insensitive
	assert.Equal(t, 2, result.RemovedCount)
}

func TestService_DeduplicateLevel2_CrossSession(t *testing.T) {
	mockRepo := newMockHashRepository()

	config := Config{
		Strategy:       StrategyUniversal,
		CleanFields:    []string{"cleanLineDescription"},
		EnableLevel2:   true,
		StoreHashes:    true,
		CaseSensitive:  false,
		TrimWhitespace: true,
	}

	service := NewService(config, mockRepo, nil)

	// First batch
	batch1ID := uuid.New()
	records1 := []Record{
		{RowIndex: 0, Data: map[string]interface{}{"cleanLineDescription": "promo tv"}},
		{RowIndex: 1, Data: map[string]interface{}{"cleanLineDescription": "revista digital"}},
	}

	result1, err := service.Deduplicate(context.Background(), batch1ID, records1)
	require.NoError(t, err)
	assert.Equal(t, 2, result1.DeduplicatedCount)
	assert.Equal(t, 0, result1.RemovedCount)

	// Second batch with overlapping data
	batch2ID := uuid.New()
	records2 := []Record{
		{RowIndex: 0, Data: map[string]interface{}{"cleanLineDescription": "promo tv"}},        // Duplicate from batch1
		{RowIndex: 1, Data: map[string]interface{}{"cleanLineDescription": "revista digital"}}, // Duplicate from batch1
		{RowIndex: 2, Data: map[string]interface{}{"cleanLineDescription": "libro mental"}},    // New
	}

	result2, err := service.Deduplicate(context.Background(), batch2ID, records2)
	require.NoError(t, err)

	assert.Equal(t, 3, result2.OriginalCount)
	assert.Equal(t, 1, result2.DeduplicatedCount) // Only "libro mental" is new
	assert.Equal(t, 2, result2.RemovedCount)      // 2 cross-session duplicates
	assert.Equal(t, 0, result2.Stats.Level1Duplicates)
	assert.Equal(t, 2, result2.Stats.Level2Duplicates)
}

func TestService_DeduplicateMultipleFields(t *testing.T) {
	config := Config{
		Strategy:       StrategyExact,
		CleanFields:    []string{"cleanLineDescription", "cleanAccount"},
		EnableLevel2:   false,
		StoreHashes:    false,
		CaseSensitive:  false,
		TrimWhitespace: true,
	}

	service := NewService(config, nil, nil)

	records := []Record{
		{
			RowIndex: 0,
			Data: map[string]interface{}{
				"cleanLineDescription": "promo tv",
				"cleanAccount":         "5000",
			},
		},
		{
			RowIndex: 1,
			Data: map[string]interface{}{
				"cleanLineDescription": "promo tv",
				"cleanAccount":         "6000", // Different account
			},
		},
		{
			RowIndex: 2,
			Data: map[string]interface{}{
				"cleanLineDescription": "promo tv",
				"cleanAccount":         "5000", // Duplicate
			},
		},
	}

	batchID := uuid.New()
	result, err := service.Deduplicate(context.Background(), batchID, records)

	require.NoError(t, err)
	assert.Equal(t, 3, result.OriginalCount)
	assert.Equal(t, 2, result.DeduplicatedCount) // Two unique combinations
	assert.Equal(t, 1, result.RemovedCount)
}

func TestService_DeduplicateEmptyRecords(t *testing.T) {
	config := DefaultConfig()
	service := NewService(config, nil, nil)

	records := []Record{}
	batchID := uuid.New()
	result, err := service.Deduplicate(context.Background(), batchID, records)

	require.NoError(t, err)
	assert.Equal(t, 0, result.OriginalCount)
	assert.Equal(t, 0, result.DeduplicatedCount)
	assert.Equal(t, 0, result.RemovedCount)
	assert.Empty(t, result.Records)
}

func TestService_DeduplicateWhitespaceHandling(t *testing.T) {
	config := Config{
		Strategy:       StrategyExact,
		CleanFields:    []string{"cleanLineDescription"},
		EnableLevel2:   false,
		StoreHashes:    false,
		CaseSensitive:  false,
		TrimWhitespace: true,
	}

	service := NewService(config, nil, nil)

	records := []Record{
		{RowIndex: 0, Data: map[string]interface{}{"cleanLineDescription": "  promo tv  "}},
		{RowIndex: 1, Data: map[string]interface{}{"cleanLineDescription": "promo tv"}},
		{RowIndex: 2, Data: map[string]interface{}{"cleanLineDescription": "  promo tv"}},
	}

	batchID := uuid.New()
	result, err := service.Deduplicate(context.Background(), batchID, records)

	require.NoError(t, err)
	assert.Equal(t, 3, result.OriginalCount)
	assert.Equal(t, 1, result.DeduplicatedCount) // All same after trimming
	assert.Equal(t, 2, result.RemovedCount)
}

func TestService_StoreHashes(t *testing.T) {
	mockRepo := newMockHashRepository()

	config := Config{
		Strategy:       StrategyExact,
		CleanFields:    []string{"cleanLineDescription"},
		EnableLevel2:   false,
		StoreHashes:    true, // Enable hash storage
		CaseSensitive:  false,
		TrimWhitespace: true,
	}

	service := NewService(config, mockRepo, nil)

	records := []Record{
		{RowIndex: 0, Data: map[string]interface{}{"cleanLineDescription": "promo tv"}},
		{RowIndex: 1, Data: map[string]interface{}{"cleanLineDescription": "promo tv"}}, // Duplicate
		{RowIndex: 2, Data: map[string]interface{}{"cleanLineDescription": "revista"}},
	}

	batchID := uuid.New()
	result, err := service.Deduplicate(context.Background(), batchID, records)

	require.NoError(t, err)
	assert.Equal(t, 2, result.DeduplicatedCount)

	// Verify hashes were stored
	savedHashes, err := mockRepo.GetBatchHashes(context.Background(), batchID)
	require.NoError(t, err)
	assert.Len(t, savedHashes, 3) // All original records

	// Count kept vs not kept
	keptCount := 0
	for _, h := range savedHashes {
		if h.Kept {
			keptCount++
		}
	}
	assert.Equal(t, 2, keptCount) // Only 2 kept (duplicates removed)
}

func TestGenerateHash_Consistency(t *testing.T) {
	config := Config{
		CaseSensitive:  false,
		TrimWhitespace: true,
	}

	record := Record{
		RowIndex: 0,
		Data: map[string]interface{}{
			"cleanLineDescription": "promo tv",
		},
	}

	fields := []string{"cleanLineDescription"}

	// Generate hash multiple times
	hash1, err := generateHash(record, fields, config)
	require.NoError(t, err)

	hash2, err := generateHash(record, fields, config)
	require.NoError(t, err)

	hash3, err := generateHash(record, fields, config)
	require.NoError(t, err)

	// All hashes should be identical
	assert.Equal(t, hash1, hash2)
	assert.Equal(t, hash2, hash3)
	assert.NotEmpty(t, hash1)
}

func TestGenerateHash_DifferentInputs(t *testing.T) {
	config := DefaultConfig()
	fields := []string{"cleanLineDescription"}

	record1 := Record{
		RowIndex: 0,
		Data:     map[string]interface{}{"cleanLineDescription": "promo tv"},
	}

	record2 := Record{
		RowIndex: 1,
		Data:     map[string]interface{}{"cleanLineDescription": "revista digital"},
	}

	hash1, err := generateHash(record1, fields, config)
	require.NoError(t, err)

	hash2, err := generateHash(record2, fields, config)
	require.NoError(t, err)

	// Different inputs should produce different hashes
	assert.NotEqual(t, hash1, hash2)
}

func BenchmarkService_Deduplicate(b *testing.B) {
	config := DefaultConfig()
	service := NewService(config, nil, nil)

	// Create 1000 records with 50% duplicates
	records := make([]Record, 1000)
	for i := 0; i < 1000; i++ {
		value := "promo tv"
		if i%2 == 0 {
			value = "revista digital"
		}
		records[i] = Record{
			RowIndex: i,
			Data: map[string]interface{}{
				"cleanLineDescription": value,
			},
		}
	}

	batchID := uuid.New()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.Deduplicate(ctx, batchID, records)
	}
}