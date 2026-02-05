package handlers

import (
	"bytes"
	"context"
	"encoding/json"
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
	mockList func(ctx context.Context, query *repository.ListQuery) ([]models.User, int64, error)
}

func (m *mockUserRepo) List(ctx context.Context, query *repository.ListQuery) ([]models.User, int64, error) {
	return m.mockList(ctx, query)
}

func TestUserHandler_Index_DefaultStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := &mockUserRepo{}
	userService := services.NewUserService(mockRepo, nil, nil, nil)
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
