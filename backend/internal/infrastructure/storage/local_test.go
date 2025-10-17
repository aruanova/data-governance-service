package storage

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestStorage(t *testing.T) (*LocalStorage, string) {
	// Create temporary directory for tests
	tempDir := t.TempDir()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Only errors in tests
	}))

	storage, err := NewLocalStorage(&LocalStorageConfig{
		BasePath: tempDir,
	}, logger)
	require.NoError(t, err)

	return storage, tempDir
}

func TestLocalStorage_SaveUpload(t *testing.T) {
	storage, _ := setupTestStorage(t)
	ctx := context.Background()

	// Test data
	fileID := "test-upload-123"
	filename := "test.csv"
	content := []byte("column1,column2\nvalue1,value2\n")

	// Save upload
	metadata, err := storage.SaveUpload(ctx, fileID, filename, bytes.NewReader(content))
	require.NoError(t, err)
	assert.NotNil(t, metadata)

	// Verify metadata
	assert.Equal(t, fileID, metadata.ID)
	assert.Equal(t, filename, metadata.OriginalName)
	assert.Equal(t, int64(len(content)), metadata.Size)
	assert.NotEmpty(t, metadata.Hash)
	assert.Equal(t, "text/csv", metadata.ContentType)
	assert.NotZero(t, metadata.CreatedAt)

	// Verify file exists
	_, err = os.Stat(metadata.StoredPath)
	assert.NoError(t, err)
}

func TestLocalStorage_GetUpload(t *testing.T) {
	storage, _ := setupTestStorage(t)
	ctx := context.Background()

	// Save a file first
	fileID := "test-upload-456"
	filename := "data.json"
	content := []byte(`{"key": "value"}`)

	_, err := storage.SaveUpload(ctx, fileID, filename, bytes.NewReader(content))
	require.NoError(t, err)

	// Retrieve the file
	reader, err := storage.GetUpload(ctx, fileID, filename)
	require.NoError(t, err)
	defer reader.Close()

	// Read content
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(reader)
	require.NoError(t, err)

	assert.Equal(t, content, buf.Bytes())
}

func TestLocalStorage_SaveProcessedFile(t *testing.T) {
	storage, _ := setupTestStorage(t)
	ctx := context.Background()

	uploadID := "batch-789"
	fileType := "cleaned"
	filename := "cleaned_data.xlsx"
	data := []byte("mock excel data")

	// Save processed file
	path, err := storage.SaveProcessedFile(ctx, uploadID, fileType, filename, data)
	require.NoError(t, err)
	assert.NotEmpty(t, path)

	// Verify file exists
	savedData, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, data, savedData)
}

func TestLocalStorage_GetProcessedFile(t *testing.T) {
	storage, _ := setupTestStorage(t)
	ctx := context.Background()

	uploadID := "batch-abc"
	fileType := "llm_input"
	filename := "input.json"
	originalData := []byte(`{"entries": []}`)

	// Save processed file
	_, err := storage.SaveProcessedFile(ctx, uploadID, fileType, filename, originalData)
	require.NoError(t, err)

	// Retrieve processed file
	data, err := storage.GetProcessedFile(ctx, uploadID, fileType, filename)
	require.NoError(t, err)
	assert.Equal(t, originalData, data)
}

func TestLocalStorage_DeleteUpload(t *testing.T) {
	storage, basePath := setupTestStorage(t)
	ctx := context.Background()

	uploadID := "delete-test-123"

	// Create some files
	_, err := storage.SaveUpload(ctx, uploadID, "test.csv", bytes.NewReader([]byte("test")))
	require.NoError(t, err)

	_, err = storage.SaveProcessedFile(ctx, uploadID, "cleaned", "clean.xlsx", []byte("cleaned"))
	require.NoError(t, err)

	// Verify directories exist
	uploadDir := filepath.Join(basePath, "uploads", uploadID)
	processedDir := filepath.Join(basePath, "processed", uploadID)

	_, err = os.Stat(uploadDir)
	assert.NoError(t, err)

	_, err = os.Stat(processedDir)
	assert.NoError(t, err)

	// Delete upload
	err = storage.DeleteUpload(ctx, uploadID)
	require.NoError(t, err)

	// Verify directories are gone
	_, err = os.Stat(uploadDir)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(processedDir)
	assert.True(t, os.IsNotExist(err))
}

func TestLocalStorage_CleanupOldFiles(t *testing.T) {
	storage, basePath := setupTestStorage(t)
	ctx := context.Background()

	// Create an old upload directory
	oldUploadID := "old-upload"
	oldDir := filepath.Join(basePath, "uploads", oldUploadID)
	err := os.MkdirAll(oldDir, 0755)
	require.NoError(t, err)

	// Set modification time to 2 hours ago
	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	err = os.Chtimes(oldDir, twoHoursAgo, twoHoursAgo)
	require.NoError(t, err)

	// Create a recent upload directory
	recentUploadID := "recent-upload"
	recentDir := filepath.Join(basePath, "uploads", recentUploadID)
	err = os.MkdirAll(recentDir, 0755)
	require.NoError(t, err)

	// Cleanup files older than 1 hour
	err = storage.CleanupOldFiles(ctx, 1*time.Hour)
	require.NoError(t, err)

	// Old directory should be deleted
	_, err = os.Stat(oldDir)
	assert.True(t, os.IsNotExist(err))

	// Recent directory should still exist
	_, err = os.Stat(recentDir)
	assert.NoError(t, err)
}

func TestLocalStorage_ListProcessedFiles(t *testing.T) {
	storage, _ := setupTestStorage(t)
	ctx := context.Background()

	uploadID := "list-test"

	// Create multiple processed files
	_, err := storage.SaveProcessedFile(ctx, uploadID, "cleaned", "cleaned.xlsx", []byte("data1"))
	require.NoError(t, err)

	_, err = storage.SaveProcessedFile(ctx, uploadID, "llm_input", "input.json", []byte("data2"))
	require.NoError(t, err)

	_, err = storage.SaveProcessedFile(ctx, uploadID, "llm_input", "input2.json", []byte("data3"))
	require.NoError(t, err)

	// List processed files
	files, err := storage.ListProcessedFiles(ctx, uploadID)
	require.NoError(t, err)

	// Verify results
	assert.Len(t, files, 2) // 2 file types: cleaned, llm_input
	assert.Contains(t, files, "cleaned")
	assert.Contains(t, files, "llm_input")
	assert.Len(t, files["cleaned"], 1)
	assert.Len(t, files["llm_input"], 2)
}

func TestLocalStorage_GetContentType(t *testing.T) {
	tests := []struct {
		filename    string
		contentType string
	}{
		{"file.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"file.xls", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"file.csv", "text/csv"},
		{"file.json", "application/json"},
		{"file.jsonl", "application/x-ndjson"},
		{"file.ndjson", "application/x-ndjson"},
		{"file.unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := getContentType(tt.filename)
			assert.Equal(t, tt.contentType, result)
		})
	}
}

func TestLocalStorage_HashConsistency(t *testing.T) {
	storage, _ := setupTestStorage(t)
	ctx := context.Background()

	content := []byte("test data for hash")

	// Save same content twice with different IDs
	meta1, err := storage.SaveUpload(ctx, "upload-1", "test.txt", bytes.NewReader(content))
	require.NoError(t, err)

	meta2, err := storage.SaveUpload(ctx, "upload-2", "test.txt", bytes.NewReader(content))
	require.NoError(t, err)

	// Hashes should be identical
	assert.Equal(t, meta1.Hash, meta2.Hash)
}