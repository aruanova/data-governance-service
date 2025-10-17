package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// LocalStorage manages file storage in local filesystem
type LocalStorage struct {
	basePath string
	logger   *slog.Logger
}

// Config for local storage
type LocalStorageConfig struct {
	BasePath string // Base directory for uploads (e.g., "/tmp/uploads")
}

// FileMetadata contains information about stored files
type FileMetadata struct {
	ID           string
	OriginalName string
	StoredPath   string
	Size         int64
	Hash         string
	ContentType  string
	CreatedAt    time.Time
}

// NewLocalStorage creates a new local storage instance
func NewLocalStorage(cfg *LocalStorageConfig, logger *slog.Logger) (*LocalStorage, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(cfg.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &LocalStorage{
		basePath: cfg.BasePath,
		logger:   logger,
	}, nil
}

// SaveUpload saves an uploaded file and returns metadata
func (s *LocalStorage) SaveUpload(ctx context.Context, fileID string, filename string, reader io.Reader) (*FileMetadata, error) {
	// Create upload-specific directory
	uploadDir := filepath.Join(s.basePath, "uploads", fileID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Sanitize filename
	safeName := filepath.Base(filename)
	destPath := filepath.Join(uploadDir, safeName)

	// Create destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Calculate hash while copying
	hash := sha256.New()
	multiWriter := io.MultiWriter(destFile, hash)

	// Copy data and calculate size
	size, err := io.Copy(multiWriter, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	fileHash := hex.EncodeToString(hash.Sum(nil))

	metadata := &FileMetadata{
		ID:           fileID,
		OriginalName: filename,
		StoredPath:   destPath,
		Size:         size,
		Hash:         fileHash,
		ContentType:  getContentType(filename),
		CreatedAt:    time.Now(),
	}

	s.logger.Info("file uploaded successfully",
		slog.String("file_id", fileID),
		slog.String("filename", filename),
		slog.Int64("size", size),
		slog.String("hash", fileHash))

	return metadata, nil
}

// GetUpload retrieves an uploaded file by ID
func (s *LocalStorage) GetUpload(ctx context.Context, fileID string, filename string) (io.ReadCloser, error) {
	filePath := filepath.Join(s.basePath, "uploads", fileID, filename)

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", fileID)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// SaveProcessedFile saves a processed file (cleaned, llm_input, etc.)
func (s *LocalStorage) SaveProcessedFile(ctx context.Context, uploadID string, fileType string, filename string, data []byte) (string, error) {
	// Create processed directory
	processedDir := filepath.Join(s.basePath, "processed", uploadID, fileType)
	if err := os.MkdirAll(processedDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create processed directory: %w", err)
	}

	filePath := filepath.Join(processedDir, filename)

	// Write data to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write processed file: %w", err)
	}

	s.logger.Info("processed file saved",
		slog.String("upload_id", uploadID),
		slog.String("type", fileType),
		slog.String("filename", filename),
		slog.Int("size", len(data)))

	return filePath, nil
}

// GetProcessedFile retrieves a processed file
func (s *LocalStorage) GetProcessedFile(ctx context.Context, uploadID string, fileType string, filename string) ([]byte, error) {
	filePath := filepath.Join(s.basePath, "processed", uploadID, fileType, filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("processed file not found: %s/%s/%s", uploadID, fileType, filename)
		}
		return nil, fmt.Errorf("failed to read processed file: %w", err)
	}

	return data, nil
}

// DeleteUpload removes all files associated with an upload
func (s *LocalStorage) DeleteUpload(ctx context.Context, uploadID string) error {
	// Delete upload directory
	uploadDir := filepath.Join(s.basePath, "uploads", uploadID)
	if err := os.RemoveAll(uploadDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete upload directory: %w", err)
	}

	// Delete processed directory
	processedDir := filepath.Join(s.basePath, "processed", uploadID)
	if err := os.RemoveAll(processedDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete processed directory: %w", err)
	}

	s.logger.Info("upload deleted",
		slog.String("upload_id", uploadID))

	return nil
}

// CleanupOldFiles removes files older than the specified duration
func (s *LocalStorage) CleanupOldFiles(ctx context.Context, olderThan time.Duration) error {
	cutoffTime := time.Now().Add(-olderThan)

	// Cleanup uploads
	uploadsDir := filepath.Join(s.basePath, "uploads")
	if err := s.cleanupDirectory(uploadsDir, cutoffTime); err != nil {
		return fmt.Errorf("failed to cleanup uploads: %w", err)
	}

	// Cleanup processed files
	processedDir := filepath.Join(s.basePath, "processed")
	if err := s.cleanupDirectory(processedDir, cutoffTime); err != nil {
		return fmt.Errorf("failed to cleanup processed files: %w", err)
	}

	s.logger.Info("cleanup completed",
		slog.Duration("older_than", olderThan))

	return nil
}

// cleanupDirectory removes directories older than cutoff time
func (s *LocalStorage) cleanupDirectory(dir string, cutoffTime time.Time) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			s.logger.Warn("failed to get file info",
				slog.String("path", dirPath),
				slog.Any("error", err))
			continue
		}

		if info.ModTime().Before(cutoffTime) {
			if err := os.RemoveAll(dirPath); err != nil {
				s.logger.Warn("failed to remove directory",
					slog.String("path", dirPath),
					slog.Any("error", err))
			} else {
				s.logger.Debug("removed old directory",
					slog.String("path", dirPath),
					slog.Time("mod_time", info.ModTime()))
			}
		}
	}

	return nil
}

// GetStoragePath returns the full path for a given upload
func (s *LocalStorage) GetStoragePath(uploadID string, fileType string) string {
	if fileType == "upload" {
		return filepath.Join(s.basePath, "uploads", uploadID)
	}
	return filepath.Join(s.basePath, "processed", uploadID, fileType)
}

// ListProcessedFiles lists all processed files for an upload
func (s *LocalStorage) ListProcessedFiles(ctx context.Context, uploadID string) (map[string][]string, error) {
	processedDir := filepath.Join(s.basePath, "processed", uploadID)

	result := make(map[string][]string)

	fileTypes, err := os.ReadDir(processedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, fmt.Errorf("failed to read processed directory: %w", err)
	}

	for _, typeEntry := range fileTypes {
		if !typeEntry.IsDir() {
			continue
		}

		fileType := typeEntry.Name()
		typeDir := filepath.Join(processedDir, fileType)

		files, err := os.ReadDir(typeDir)
		if err != nil {
			continue
		}

		var fileNames []string
		for _, file := range files {
			if !file.IsDir() {
				fileNames = append(fileNames, file.Name())
			}
		}

		if len(fileNames) > 0 {
			result[fileType] = fileNames
		}
	}

	return result, nil
}

// getContentType returns the content type based on file extension
func getContentType(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".xlsx", ".xls":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".jsonl", ".ndjson":
		return "application/x-ndjson"
	default:
		return "application/octet-stream"
	}
}