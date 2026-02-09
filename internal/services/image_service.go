package services

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
)

// ImageService handles image processing and storage
type ImageService struct {
	uploadDir string
}

func NewImageService(uploadDir string) *ImageService {
	// Ensure upload directory exists
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		_ = os.MkdirAll(uploadDir, 0755)
	}
	return &ImageService{
		uploadDir: uploadDir,
	}
}

// ProcessAndSaveProfilePicture saves the original image and creates a thumbnail
func (s *ImageService) ProcessAndSaveProfilePicture(file multipart.File, header *multipart.FileHeader) (originalPath, thumbnailPath string, err error) {
	// Validate file type
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		return "", "", fmt.Errorf("formato de imagen no soportado (solo JPG/PNG)")
	}

	// Generate unique filename
	filename := uuid.New().String()
	originalFilename := filename + ext
	thumbFilename := filename + "_thumb" + ext

	// Create relative paths for DB/Response (assuming /uploads is served statically)
	// We'll return paths relative to the upload root, to be prefixed or served as is
	originalRelPath := "/uploads/" + originalFilename
	thumbRelPath := "/uploads/" + thumbFilename

	// Decode image
	img, _, err := image.Decode(file)
	if err != nil {
		return "", "", fmt.Errorf("error al decodificar imagen: %w", err)
	}

	// Save original (reset seek or decode again? No, we have the image object)
	// Actually, for original, we might want to just copy the file stream to disk to avoid re-encoding loss/overhead if we don't need to resize it.
	// But validating it by decoding is good. Let's re-encode to ensure consistency or just copy.
	// Let's copy the original stream to disk to preserve original quality/metadata if desired,
	// BUT we already decoded it.
	// To copy, we need to seek back to 0.
	if _, err := file.Seek(0, 0); err != nil {
		return "", "", fmt.Errorf("error al leer archivo: %w", err)
	}

	outOriginalPath := filepath.Join(s.uploadDir, originalFilename)
	outOriginal, err := os.Create(outOriginalPath)
	if err != nil {
		return "", "", fmt.Errorf("error al crear archivo: %w", err)
	}
	defer outOriginal.Close()

	if _, err := io.Copy(outOriginal, file); err != nil {
		return "", "", fmt.Errorf("error al guardar imagen original: %w", err)
	}

	// Create thumbnail
	// Resize to 128x128 fit (or fill?)
	// Profile pictures usually generally square-ish.
	// Let's use Fill to force square, or Fit to keep aspect ratio.
	// Fill is better for avatars.
	thumbImg := imaging.Fill(img, 128, 128, imaging.Center, imaging.Lanczos)

	outThumbPath := filepath.Join(s.uploadDir, thumbFilename)
	outThumb, err := os.Create(outThumbPath)
	if err != nil {
		return "", "", fmt.Errorf("error al crear thumbnail: %w", err)
	}
	defer outThumb.Close()

	// Encode thumbnail
	if ext == ".png" {
		err = png.Encode(outThumb, thumbImg)
	} else {
		err = jpeg.Encode(outThumb, thumbImg, &jpeg.Options{Quality: 85})
	}

	if err != nil {
		return "", "", fmt.Errorf("error al guardar thumbnail: %w", err)
	}

	// Add timestamp query param to bust cache if needed, but for now just path
	// or maybe returning just the filename and letting frontend construct URL?
	// The plan said "Return relative paths".
	return originalRelPath + "?t=" + fmt.Sprintf("%d", time.Now().Unix()), thumbRelPath + "?t=" + fmt.Sprintf("%d", time.Now().Unix()), nil
}
