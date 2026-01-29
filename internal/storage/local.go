package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"
)

// LocalStorage handles file storage on the local filesystem
type LocalStorage struct {
	basePath string
}

// NewLocalStorage creates a new local storage instance
func NewLocalStorage(basePath string) (*LocalStorage, error) {
	// Ensure the base directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	return &LocalStorage{basePath: basePath}, nil
}

// Upload saves a file and returns its relative path
func (s *LocalStorage) Upload(file multipart.File, header *multipart.FileHeader, subDir string) (string, error) {
	// Create subdirectory organized by year/month (e.g., "contracts/2026/01")
	dir := filepath.Join(s.basePath, subDir, time.Now().Format("2006/01"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate unique filename
	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%s%s", generateID(), ext)
	filePath := filepath.Join(dir, filename)

	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Copy content
	if _, err := io.Copy(dst, file); err != nil {
		// Clean up on failure
		os.Remove(filePath)
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	// Return relative path for database storage
	relPath, _ := filepath.Rel(s.basePath, filePath)
	return relPath, nil
}

// UploadFromBytes saves bytes to a file and returns its relative path
func (s *LocalStorage) UploadFromBytes(data []byte, filename string, subDir string) (string, error) {
	dir := filepath.Join(s.basePath, subDir, time.Now().Format("2006/01"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	ext := filepath.Ext(filename)
	uniqueFilename := fmt.Sprintf("%s%s", generateID(), ext)
	filePath := filepath.Join(dir, uniqueFilename)

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	relPath, _ := filepath.Rel(s.basePath, filePath)
	return relPath, nil
}

// Download returns a file for reading
func (s *LocalStorage) Download(relativePath string) (*os.File, error) {
	filePath := filepath.Join(s.basePath, relativePath)
	return os.Open(filePath)
}

// Delete removes a file
func (s *LocalStorage) Delete(relativePath string) error {
	filePath := filepath.Join(s.basePath, relativePath)
	return os.Remove(filePath)
}

// Exists checks if a file exists
func (s *LocalStorage) Exists(relativePath string) bool {
	filePath := filepath.Join(s.basePath, relativePath)
	_, err := os.Stat(filePath)
	return err == nil
}

// GetFullPath returns the absolute path for serving files
func (s *LocalStorage) GetFullPath(relativePath string) string {
	return filepath.Join(s.basePath, relativePath)
}

// GetSize returns the size of a file in bytes
func (s *LocalStorage) GetSize(relativePath string) (int64, error) {
	filePath := filepath.Join(s.basePath, relativePath)
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// generateID creates a unique identifier for filenames
func generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// ValidContentTypes returns allowed MIME types for uploads
func ValidContentTypes() map[string]bool {
	return map[string]bool{
		"application/pdf": true,
		"image/jpeg":      true,
		"image/jpg":       true,
		"image/png":       true,
	}
}

// MaxFileSize returns the maximum allowed file size (10MB)
func MaxFileSize() int64 {
	return 10 * 1024 * 1024 // 10 MB
}

// IsValidContentType checks if the content type is allowed
func IsValidContentType(contentType string) bool {
	return ValidContentTypes()[contentType]
}
