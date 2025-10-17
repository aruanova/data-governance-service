package deduplication

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Service implements the Deduplicator interface
type Service struct {
	config   Config
	hashRepo HashRepository
	logger   *slog.Logger
}

// NewService creates a new deduplication service
func NewService(config Config, hashRepo HashRepository, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		config:   config,
		hashRepo: hashRepo,
		logger:   logger,
	}
}

// Deduplicate performs two-level deduplication
func (s *Service) Deduplicate(ctx context.Context, batchID uuid.UUID, records []Record) (*DeduplicationResult, error) {
	startTime := time.Now()

	s.logger.Info("starting deduplication",
		slog.String("batch_id", batchID.String()),
		slog.Int("record_count", len(records)),
		slog.String("strategy", string(s.config.Strategy)))

	if len(records) == 0 {
		return &DeduplicationResult{
			OriginalCount:    0,
			DeduplicatedCount: 0,
			RemovedCount:     0,
			Strategy:         s.config.Strategy,
			Records:          []Record{},
			Stats:            DeduplicationStats{},
		}, nil
	}

	// Generate hashes for all records
	if err := s.generateHashes(records); err != nil {
		return nil, fmt.Errorf("failed to generate hashes: %w", err)
	}

	// Level 1: Within-batch deduplication
	level1Result, err := s.deduplicateLevel1(ctx, records)
	if err != nil {
		return nil, fmt.Errorf("level 1 deduplication failed: %w", err)
	}

	s.logger.Info("level 1 deduplication completed",
		slog.Int("duplicates_removed", level1Result.RemovedCount))

	// Level 2: Cross-session deduplication (if enabled)
	finalRecords := level1Result.Records
	level2Duplicates := 0

	if s.config.EnableLevel2 && s.hashRepo != nil {
		level2Result, err := s.deduplicateLevel2(ctx, batchID, finalRecords)
		if err != nil {
			s.logger.Error("level 2 deduplication failed", "error", err)
			// Continue with level 1 results if level 2 fails
		} else {
			finalRecords = level2Result.Records
			level2Duplicates = level2Result.RemovedCount

			s.logger.Info("level 2 deduplication completed",
				slog.Int("duplicates_removed", level2Duplicates))
		}
	}

	// Store hashes in database if configured
	if s.config.StoreHashes && s.hashRepo != nil {
		if err := s.storeHashes(ctx, batchID, records, finalRecords); err != nil {
			s.logger.Error("failed to store hashes", "error", err)
			// Don't fail the entire operation if hash storage fails
		}
	}

	processingTime := time.Since(startTime).Milliseconds()

	result := &DeduplicationResult{
		OriginalCount:    len(records),
		DeduplicatedCount: len(finalRecords),
		RemovedCount:     len(records) - len(finalRecords),
		Strategy:         s.config.Strategy,
		Records:          finalRecords,
		Stats: DeduplicationStats{
			Level1Duplicates: level1Result.RemovedCount,
			Level2Duplicates: level2Duplicates,
			UniqueRecords:    len(finalRecords),
			ProcessingTimeMs: processingTime,
		},
	}

	s.logger.Info("deduplication completed",
		slog.Int("original_count", result.OriginalCount),
		slog.Int("final_count", result.DeduplicatedCount),
		slog.Int("removed_count", result.RemovedCount),
		slog.Int64("processing_time_ms", processingTime))

	return result, nil
}

// deduplicateLevel1 performs within-batch deduplication
func (s *Service) deduplicateLevel1(ctx context.Context, records []Record) (*DeduplicationResult, error) {
	seen := make(map[string]bool)
	unique := make([]Record, 0, len(records))
	duplicates := 0

	for _, record := range records {
		if record.Hash == "" {
			s.logger.Warn("record without hash, skipping",
				slog.Int("row_index", record.RowIndex))
			continue
		}

		if !seen[record.Hash] {
			seen[record.Hash] = true
			unique = append(unique, record)
		} else {
			duplicates++
			s.logger.Debug("level 1 duplicate found",
				slog.String("hash", record.Hash),
				slog.Int("row_index", record.RowIndex))
		}
	}

	return &DeduplicationResult{
		OriginalCount:    len(records),
		DeduplicatedCount: len(unique),
		RemovedCount:     duplicates,
		Records:          unique,
	}, nil
}

// deduplicateLevel2 performs cross-session deduplication
func (s *Service) deduplicateLevel2(ctx context.Context, batchID uuid.UUID, records []Record) (*DeduplicationResult, error) {
	if s.hashRepo == nil {
		return &DeduplicationResult{
			Records:      records,
			RemovedCount: 0,
		}, nil
	}

	unique := make([]Record, 0, len(records))
	duplicates := 0

	for _, record := range records {
		// Check if hash exists in previous batches
		exists, err := s.hashRepo.CheckHashExists(ctx, record.Hash)
		if err != nil {
			s.logger.Error("failed to check hash existence",
				slog.String("hash", record.Hash),
				"error", err)
			// On error, keep the record (fail-open)
			unique = append(unique, record)
			continue
		}

		if !exists {
			// Hash is unique across all batches
			unique = append(unique, record)
		} else {
			duplicates++
			s.logger.Debug("level 2 duplicate found (cross-session)",
				slog.String("hash", record.Hash),
				slog.Int("row_index", record.RowIndex))
		}
	}

	return &DeduplicationResult{
		OriginalCount:    len(records),
		DeduplicatedCount: len(unique),
		RemovedCount:     duplicates,
		Records:          unique,
	}, nil
}

// generateHashes generates hashes for all records
func (s *Service) generateHashes(records []Record) error {
	for i := range records {
		hash, err := generateHash(records[i], s.config.CleanFields, s.config)
		if err != nil {
			return fmt.Errorf("failed to hash record %d: %w", i, err)
		}
		records[i].Hash = hash
	}
	return nil
}

// storeHashes stores deduplication hashes in the database
func (s *Service) storeHashes(ctx context.Context, batchID uuid.UUID, original, final []Record) error {
	// Create a set of kept row indices
	keptIndices := make(map[int]bool)
	for _, record := range final {
		keptIndices[record.RowIndex] = true
	}

	// Create hash entries
	entries := make([]HashEntry, 0, len(original))
	for _, record := range original {
		entries = append(entries, HashEntry{
			Hash:             record.Hash,
			OriginalRowIndex: record.RowIndex,
			Kept:             keptIndices[record.RowIndex],
		})
	}

	return s.hashRepo.SaveHashes(ctx, batchID, entries)
}

// GetConfig returns the current configuration
func (s *Service) GetConfig() Config {
	return s.config
}