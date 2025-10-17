package repositories

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/alejandroruanova/data-governance-service/backend/internal/core/domain"
	"github.com/alejandroruanova/data-governance-service/backend/internal/core/services/deduplication"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DedupHashRepository implements the HashRepository interface using GORM
type DedupHashRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewDedupHashRepository creates a new repository instance
func NewDedupHashRepository(db *gorm.DB, logger *slog.Logger) *DedupHashRepository {
	if logger == nil {
		logger = slog.Default()
	}

	return &DedupHashRepository{
		db:     db,
		logger: logger,
	}
}

// CheckHashExists verifies if a hash exists for any batch (universal deduplication)
func (r *DedupHashRepository) CheckHashExists(ctx context.Context, hash string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Model(&domain.DedupHash{}).
		Where("hash = ? AND kept = ?", hash, true).
		Count(&count).
		Error

	if err != nil {
		r.logger.Error("failed to check hash existence",
			slog.String("hash", hash),
			slog.Error(err))
		return false, fmt.Errorf("database query failed: %w", err)
	}

	return count > 0, nil
}

// SaveHashes stores deduplication hashes for a batch
func (r *DedupHashRepository) SaveHashes(ctx context.Context, batchID uuid.UUID, hashes []deduplication.HashEntry) error {
	if len(hashes) == 0 {
		return nil
	}

	// Convert to domain models
	dedupHashes := make([]domain.DedupHash, 0, len(hashes))
	for _, entry := range hashes {
		dedupHashes = append(dedupHashes, domain.DedupHash{
			ID:               uuid.New(),
			BatchID:          batchID,
			Hash:             entry.Hash,
			OriginalRowIndex: entry.OriginalRowIndex,
			Kept:             entry.Kept,
		})
	}

	// Batch insert for better performance
	err := r.db.WithContext(ctx).
		CreateInBatches(dedupHashes, 1000).
		Error

	if err != nil {
		r.logger.Error("failed to save hashes",
			slog.String("batch_id", batchID.String()),
			slog.Int("hash_count", len(hashes)),
			slog.Error(err))
		return fmt.Errorf("failed to insert hashes: %w", err)
	}

	r.logger.Info("saved deduplication hashes",
		slog.String("batch_id", batchID.String()),
		slog.Int("hash_count", len(hashes)))

	return nil
}

// GetBatchHashes retrieves all hashes for a specific batch
func (r *DedupHashRepository) GetBatchHashes(ctx context.Context, batchID uuid.UUID) ([]deduplication.HashEntry, error) {
	var dedupHashes []domain.DedupHash

	err := r.db.WithContext(ctx).
		Where("batch_id = ?", batchID).
		Order("original_row_index ASC").
		Find(&dedupHashes).
		Error

	if err != nil {
		r.logger.Error("failed to get batch hashes",
			slog.String("batch_id", batchID.String()),
			slog.Error(err))
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	// Convert to HashEntry
	entries := make([]deduplication.HashEntry, 0, len(dedupHashes))
	for _, dh := range dedupHashes {
		entries = append(entries, deduplication.HashEntry{
			Hash:             dh.Hash,
			OriginalRowIndex: dh.OriginalRowIndex,
			Kept:             dh.Kept,
		})
	}

	return entries, nil
}

// DeleteBatchHashes removes all hashes for a specific batch
func (r *DedupHashRepository) DeleteBatchHashes(ctx context.Context, batchID uuid.UUID) error {
	err := r.db.WithContext(ctx).
		Where("batch_id = ?", batchID).
		Delete(&domain.DedupHash{}).
		Error

	if err != nil {
		r.logger.Error("failed to delete batch hashes",
			slog.String("batch_id", batchID.String()),
			slog.Error(err))
		return fmt.Errorf("failed to delete hashes: %w", err)
	}

	r.logger.Info("deleted batch hashes",
		slog.String("batch_id", batchID.String()))

	return nil
}

// GetDuplicateCount returns the number of duplicates found for a batch
func (r *DedupHashRepository) GetDuplicateCount(ctx context.Context, batchID uuid.UUID) (int64, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Model(&domain.DedupHash{}).
		Where("batch_id = ? AND kept = ?", batchID, false).
		Count(&count).
		Error

	if err != nil {
		r.logger.Error("failed to get duplicate count",
			slog.String("batch_id", batchID.String()),
			slog.Error(err))
		return 0, fmt.Errorf("database query failed: %w", err)
	}

	return count, nil
}

// GetHashDistribution returns statistics about hash distribution
func (r *DedupHashRepository) GetHashDistribution(ctx context.Context, batchID uuid.UUID) (map[string]int, error) {
	type hashCount struct {
		Hash  string
		Count int64
	}

	var results []hashCount

	err := r.db.WithContext(ctx).
		Model(&domain.DedupHash{}).
		Select("hash, COUNT(*) as count").
		Where("batch_id = ?", batchID).
		Group("hash").
		Having("COUNT(*) > 1"). // Only duplicates
		Scan(&results).
		Error

	if err != nil {
		r.logger.Error("failed to get hash distribution",
			slog.String("batch_id", batchID.String()),
			slog.Error(err))
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	distribution := make(map[string]int)
	for _, result := range results {
		distribution[result.Hash] = int(result.Count)
	}

	return distribution, nil
}