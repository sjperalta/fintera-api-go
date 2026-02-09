package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/stretchr/testify/assert"
)

func TestCreateUserRequestBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		payload        map[string]interface{}
		expectedStatus int
		expectedName   string
		expectError    bool
	}{
		{
			name: "Support full_name (snake_case)",
			payload: map[string]interface{}{
				"email":     "test@example.com",
				"password":  "password123",
				"full_name": "Snake Case User",
			},
			expectedStatus: http.StatusOK, // We'll mock the rest so it doesn't fail on service call
			expectedName:   "Snake Case User",
		},
		{
			name: "Support FullName (PascalCase)",
			payload: map[string]interface{}{
				"email":    "test@example.com",
				"password": "password123",
				"FullName": "Pascal Case User",
			},
			expectedStatus: http.StatusOK,
			expectedName:   "Pascal Case User",
		},
		{
			name: "Error when both missing",
			payload: map[string]interface{}{
				"email":    "test@example.com",
				"password": "password123",
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			jsonBytes, _ := json.Marshal(tt.payload)
			c.Request, _ = http.NewRequest("POST", "/users", bytes.NewBuffer(jsonBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			var req CreateUserRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				if !tt.expectError {
					t.Errorf("Unexpected error binding: %v", err)
				}
				return
			}

			if req.FullName == "" && req.FullNamePascal != "" {
				req.FullName = req.FullNamePascal
			}

			if tt.expectError {
				if req.FullName != "" {
					t.Errorf("Expected validation error but got FullName: %s", req.FullName)
				}
			} else {
				assert.Equal(t, tt.expectedName, req.FullName)
			}
		})
	}
}

type mockUserRepo struct {
	repository.UserRepository
	mockList     func(ctx context.Context, query *repository.ListQuery) ([]models.User, int64, error)
	mockFindByID func(ctx context.Context, id uint) (*models.User, error)
	mockUpdate   func(ctx context.Context, user *models.User) error
}

func (m *mockUserRepo) List(ctx context.Context, query *repository.ListQuery) ([]models.User, int64, error) {
	if m.mockList != nil {
		return m.mockList(ctx, query)
	}
	return []models.User{}, 0, nil
}

func (m *mockUserRepo) FindByID(ctx context.Context, id uint) (*models.User, error) {
	if m.mockFindByID != nil {
		return m.mockFindByID(ctx, id)
	}
	return nil, nil
}

func (m *mockUserRepo) Update(ctx context.Context, user *models.User) error {
	if m.mockUpdate != nil {
		return m.mockUpdate(ctx, user)
	}
	return nil
}

func TestUserHandler_Index_DefaultStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := &mockUserRepo{}
	userService := services.NewUserService(mockRepo, nil, nil, nil, nil, nil) // Updated with nil ContractRepo and ImageService
	handler := NewUserHandler(userService, nil)

	var capturedStatus string
	mockRepo.mockList = func(ctx context.Context, query *repository.ListQuery) ([]models.User, int64, error) {
		capturedStatus = query.Filters["status"]
		return []models.User{}, 0, nil
	}

	// Test Case 1: No status provided -> should default to "active"
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/users", nil)
	handler.Index(c)
	assert.Equal(t, models.StatusActive, capturedStatus)

	// Test Case 2: Status "all" provided -> should be empty string (no filter)
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/users?status=all", nil)
	handler.Index(c)
	assert.Equal(t, "", capturedStatus)

	// Test Case 3: Specific status provided -> should use it
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/users?status=inactive", nil)
	handler.Index(c)
	assert.Equal(t, "inactive", capturedStatus)
}

func TestUploadProfilePicture(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create temp dir for uploads
	tempDir := t.TempDir()
	imageService := services.NewImageService(tempDir)

	mockRepo := &mockUserRepo{}
	userService := services.NewUserService(mockRepo, nil, nil, nil, nil, imageService)
	handler := NewUserHandler(userService, nil)

	userID := uint(1)
	mockUser := &models.User{ID: userID, Email: "test@example.com"}

	mockRepo.mockFindByID = func(ctx context.Context, id uint) (*models.User, error) {
		if id == userID {
			return mockUser, nil
		}
		return nil, nil
	}

	mockRepo.mockUpdate = func(ctx context.Context, user *models.User) error {
		assert.NotNil(t, user.ProfilePicture)
		assert.NotNil(t, user.ProfilePictureThumb)
		return nil
	}

	// Mock file upload
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Minimal valid 1x1 PNG
	minimalPNG := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // Magic
		0x00, 0x00, 0x00, 0x0d, // IHDR length
		0x49, 0x48, 0x44, 0x52, // IHDR chunk type
		0x00, 0x00, 0x00, 0x01, // Width: 1
		0x00, 0x00, 0x00, 0x01, // Height: 1
		0x08, 0x06, 0x00, 0x00, 0x00, // Bit depth, Color type, Compression, Filter, Interlace
		0x1f, 0x15, 0xc4, 0x89, // CRC
		0x00, 0x00, 0x00, 0x0a, // IDAT length
		0x49, 0x44, 0x41, 0x54, // IDAT chunk type
		0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, // Compressed data
		0x0d, 0x0a, 0x2d, 0xb4, // CRC
		0x00, 0x00, 0x00, 0x00, // IEND length
		0x49, 0x45, 0x4e, 0x44, // IEND chunk type
		0xae, 0x42, 0x60, 0x82, // CRC
	}
	part, err := writer.CreateFormFile("image", "test.png")
	assert.NoError(t, err)
	part.Write(minimalPNG)
	writer.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/users/1/picture", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	c.Params = []gin.Param{{Key: "user_id", Value: "1"}}
	c.Set("userID", uint(1)) // Mock authenticated user (self update)

	handler.UploadProfilePicture(c)

	assert.Equal(t, http.StatusOK, w.Code)
}
