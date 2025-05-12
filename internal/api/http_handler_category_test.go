// File: product-catalog-service/internal/api/http_handler_category_test.go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	// "strconv" // Not directly used in these category tests after refactor
	"testing"
	"time"

	"product-catalog-service/internal/domain" // Corrected import
	"product-catalog-service/internal/store"  // Corrected import
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"    // For mocking the store interface
	"github.com/stretchr/testify/require"
	"github.com/go-chi/chi/v5" // For setting up the router in tests
)

// MockCategoryStorer is a mock implementation of store.CategoryStorer
type MockCategoryStorer struct {
	mock.Mock
}

func (m *MockCategoryStorer) CreateCategory(ctx context.Context, category *domain.Category) (*domain.Category, error) {
	args := m.Called(ctx, category)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Category), args.Error(1)
}

func (m *MockCategoryStorer) GetCategoryByID(ctx context.Context, id int64) (*domain.Category, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Category), args.Error(1)
}

func (m *MockCategoryStorer) ListCategories(ctx context.Context, params store.ListCategoriesParams) ([]domain.Category, int, error) {
	args := m.Called(ctx, params)
	var categories []domain.Category
	if arg0 := args.Get(0); arg0 != nil {
		categories = arg0.([]domain.Category)
	}
	return categories, args.Int(1), args.Error(2)
}

func (m *MockCategoryStorer) UpdateCategory(ctx context.Context, category *domain.Category) (*domain.Category, error) {
	args := m.Called(ctx, category)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Category), args.Error(1)
}

func (m *MockCategoryStorer) DeleteCategory(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Helper for setting up tests with a chi router and handler
func setupTestChiServer(t *testing.T, cs store.CategoryStorer, ps store.ProductStorer) *httptest.Server {
	handler := NewHTTPHandler(cs, ps) // Assuming NewHTTPHandler takes both, pass nil for productStore if not used
	router := chi.NewRouter()
	handler.RegisterRoutes(router) // Use the unified RegisterRoutes method

	return httptest.NewServer(router)
}

// Helper function to get a pointer (useful for optional fields in domain structs)
// Consider moving to a common test helper package if used across multiple test files.
func PtrTo[T any](v T) *T {
	return &v
}

func TestHTTPHandler_CreateCategory_Success(t *testing.T) {
	mockCatStore := new(MockCategoryStorer)
	// For category-specific tests, productStore can be nil if not used by category handlers
	server := setupTestChiServer(t, mockCatStore, nil)
	defer server.Close()

	now := time.Now().Truncate(time.Millisecond)
	inputPayload := CategoryCreateInput{ // Using the actual input struct
		Name:        "New API Test Category",
		Description: PtrTo("Description for API category"),
	}
	expectedCreatedCategory := &domain.Category{
		ID:          1, // Assuming store assigns an ID
		Name:        inputPayload.Name,
		Description: inputPayload.Description,
		CreatedAt:   now, // Store would set these
		UpdatedAt:   now, // Store would set these
	}

	// Mock the CreateCategory call
	mockCatStore.On("CreateCategory", mock.Anything, mock.MatchedBy(func(cat *domain.Category) bool {
		return cat.Name == inputPayload.Name && (cat.Description != nil && *cat.Description == *inputPayload.Description)
	})).Return(expectedCreatedCategory, nil).Once()

	reqBody, _ := json.Marshal(inputPayload)
	res, err := http.Post(server.URL+"/api/v1/categories", "application/json", bytes.NewBuffer(reqBody))
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, http.StatusCreated, res.StatusCode)

	var responseCategory domain.Category
	err = json.NewDecoder(res.Body).Decode(&responseCategory)
	require.NoError(t, err)
	assert.Equal(t, expectedCreatedCategory.ID, responseCategory.ID)
	assert.Equal(t, expectedCreatedCategory.Name, responseCategory.Name)
	require.NotNil(t, responseCategory.Description)
	require.NotNil(t, expectedCreatedCategory.Description)
	assert.Equal(t, *expectedCreatedCategory.Description, *responseCategory.Description)
	// Timestamps are set by the store, so we check if they are recent enough for the test
	assert.WithinDuration(t, now, responseCategory.CreatedAt, time.Second*5) // Allow some leeway
	assert.WithinDuration(t, now, responseCategory.UpdatedAt, time.Second*5)


	mockCatStore.AssertExpectations(t)
}

func TestHTTPHandler_CreateCategory_InvalidPayload_Validation(t *testing.T) {
	mockCatStore := new(MockCategoryStorer)
	server := setupTestChiServer(t, mockCatStore, nil)
	defer server.Close()

	// Example: Name is required, send empty name
	inputPayload := CategoryCreateInput{Name: ""}
	reqBody, _ := json.Marshal(inputPayload)

	res, err := http.Post(server.URL+"/api/v1/categories", "application/json", bytes.NewBuffer(reqBody))
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	var errResp ErrorResponse // Using the ErrorResponse struct from http_handler.go
	err = json.NewDecoder(res.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp.Error, "Validation failed", "Error message should indicate validation failure")
	// The exact error message from validator/v10 can be complex, so checking for prefix is safer
}

func TestHTTPHandler_CreateCategory_StoreError_NameExists(t *testing.T) {
	mockCatStore := new(MockCategoryStorer)
	server := setupTestChiServer(t, mockCatStore, nil)
	defer server.Close()

	inputPayload := CategoryCreateInput{Name: "Existing Name", Description: PtrTo("Desc")}

	// Mock the store to return ErrCategoryNameExists
	mockCatStore.On("CreateCategory", mock.Anything, mock.AnythingOfType("*domain.Category")).
		Return(nil, store.ErrCategoryNameExists).Once()

	reqBody, _ := json.Marshal(inputPayload)
	res, err := http.Post(server.URL+"/api/v1/categories", "application/json", bytes.NewBuffer(reqBody))
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusConflict, res.StatusCode)
	var errResp ErrorResponse
	err = json.NewDecoder(res.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, store.ErrCategoryNameExists.Error(), errResp.Error)

	mockCatStore.AssertExpectations(t)
}


func TestHTTPHandler_ListCategories_Success(t *testing.T) {
	mockCatStore := new(MockCategoryStorer)
	server := setupTestChiServer(t, mockCatStore, nil)
	defer server.Close()

	now := time.Now().Truncate(time.Millisecond)
	expectedCategories := []domain.Category{
		{ID: 1, Name: "Cat A", CreatedAt: now, UpdatedAt: now},
		{ID: 2, Name: "Cat B", CreatedAt: now, UpdatedAt: now},
	}
	expectedTotalCount := 2 // This is an int

	// Mock the ListCategories call
	mockCatStore.On("ListCategories", mock.Anything, store.ListCategoriesParams{Limit: 10, Offset: 0}).
		Return(expectedCategories, expectedTotalCount, nil).Once()

	res, err := http.Get(server.URL + "/api/v1/categories?page=1&limit=10")
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)

	var responsePayload struct {
		Data       []domain.Category `json:"data"`
		Pagination struct {
			Page       int `json:"page"`
			Limit      int `json:"limit"`
			TotalItems int `json:"total_items"`
			TotalPages int `json:"total_pages"`
		} `json:"pagination"`
	}
	err = json.NewDecoder(res.Body).Decode(&responsePayload)
	require.NoError(t, err)

	assert.Len(t, responsePayload.Data, 2)
	assert.Equal(t, "Cat A", responsePayload.Data[0].Name)
	assert.Equal(t, 1, responsePayload.Pagination.Page)
	assert.Equal(t, 10, responsePayload.Pagination.Limit)
	assert.Equal(t, expectedTotalCount, responsePayload.Pagination.TotalItems)
	assert.Equal(t, 1, responsePayload.Pagination.TotalPages) // (2 + 10 - 1) / 10 = 1

	mockCatStore.AssertExpectations(t)
}


func TestHTTPHandler_GetCategoryByID_Found(t *testing.T) {
	mockCatStore := new(MockCategoryStorer)
	server := setupTestChiServer(t, mockCatStore, nil)
	defer server.Close()

	categoryID := int64(1)
	now := time.Now().Truncate(time.Millisecond)
	expectedCategory := &domain.Category{
		ID: categoryID, Name: "Fetched Category", Description: PtrTo("Details"), CreatedAt: now, UpdatedAt: now,
	}

	mockCatStore.On("GetCategoryByID", mock.Anything, categoryID).Return(expectedCategory, nil).Once()

	res, err := http.Get(server.URL + fmt.Sprintf("/api/v1/categories/%d", categoryID))
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)
	var responseCategory domain.Category
	err = json.NewDecoder(res.Body).Decode(&responseCategory)
	require.NoError(t, err)
	assert.Equal(t, expectedCategory.ID, responseCategory.ID)
	assert.Equal(t, expectedCategory.Name, responseCategory.Name)

	mockCatStore.AssertExpectations(t)
}

func TestHTTPHandler_GetCategoryByID_NotFound(t *testing.T) {
	mockCatStore := new(MockCategoryStorer)
	server := setupTestChiServer(t, mockCatStore, nil)
	defer server.Close()

	categoryID := int64(99)
	// Mock the store to return the specific ErrCategoryNotFound
	mockCatStore.On("GetCategoryByID", mock.Anything, categoryID).Return(nil, store.ErrCategoryNotFound).Once()

	res, err := http.Get(server.URL + fmt.Sprintf("/api/v1/categories/%d", categoryID))
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	var errResp ErrorResponse
	err = json.NewDecoder(res.Body).Decode(&errResp)
	require.NoError(t, err)
	// Check against the error message produced by the handler, which uses the store's error
	assert.Equal(t, store.ErrCategoryNotFound.Error(), errResp.Error)

	mockCatStore.AssertExpectations(t)
}

func TestHTTPHandler_UpdateCategory_Success(t *testing.T) {
	mockCatStore := new(MockCategoryStorer)
	server := setupTestChiServer(t, mockCatStore, nil)
	defer server.Close()

	categoryID := int64(1)
	now := time.Now().Truncate(time.Millisecond)
	updatePayload := CategoryUpdateInput{ // Using the actual input struct
		Name:        "Updated Category Name",
		Description: PtrTo("Updated Description"),
	}
	expectedUpdatedCategory := &domain.Category{
		ID:          categoryID,
		Name:        updatePayload.Name,
		Description: updatePayload.Description,
		UpdatedAt:   now,                     // Store would set this
		CreatedAt:   now.Add(-time.Hour),     // Assume an original CreatedAt
	}

	mockCatStore.On("UpdateCategory", mock.Anything, mock.MatchedBy(func(cat *domain.Category) bool {
		return cat.ID == categoryID && cat.Name == updatePayload.Name
	})).Return(expectedUpdatedCategory, nil).Once()

	reqBody, _ := json.Marshal(updatePayload)
	req, err := http.NewRequest(http.MethodPut, server.URL+fmt.Sprintf("/api/v1/categories/%d", categoryID), bytes.NewBuffer(reqBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)
	var responseCategory domain.Category
	err = json.NewDecoder(res.Body).Decode(&responseCategory)
	require.NoError(t, err)
	assert.Equal(t, expectedUpdatedCategory.Name, responseCategory.Name)
	require.NotNil(t, responseCategory.Description)
	require.NotNil(t, expectedUpdatedCategory.Description)
	assert.Equal(t, *expectedUpdatedCategory.Description, *responseCategory.Description)
	assert.WithinDuration(t, now, responseCategory.UpdatedAt, time.Second*5)

	mockCatStore.AssertExpectations(t)
}

func TestHTTPHandler_UpdateCategory_NotFound(t *testing.T) {
	mockCatStore := new(MockCategoryStorer)
	server := setupTestChiServer(t, mockCatStore, nil)
	defer server.Close()

	categoryID := int64(99) // Non-existent ID
	updatePayload := CategoryUpdateInput{Name: "Non Existent Update"}

	mockCatStore.On("UpdateCategory", mock.Anything, mock.MatchedBy(func(cat *domain.Category) bool {
		return cat.ID == categoryID
	})).Return(nil, store.ErrCategoryNotFound).Once()

	reqBody, _ := json.Marshal(updatePayload)
	req, err := http.NewRequest(http.MethodPut, server.URL+fmt.Sprintf("/api/v1/categories/%d", categoryID), bytes.NewBuffer(reqBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	var errResp ErrorResponse
	err = json.NewDecoder(res.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, store.ErrCategoryNotFound.Error(), errResp.Error)

	mockCatStore.AssertExpectations(t)
}


func TestHTTPHandler_DeleteCategory_Success(t *testing.T) {
	mockCatStore := new(MockCategoryStorer)
	server := setupTestChiServer(t, mockCatStore, nil)
	defer server.Close()

	categoryID := int64(1)

	mockCatStore.On("DeleteCategory", mock.Anything, categoryID).Return(nil).Once()

	req, err := http.NewRequest(http.MethodDelete, server.URL+fmt.Sprintf("/api/v1/categories/%d", categoryID), nil)
	require.NoError(t, err)

	client := &http.Client{}
	res, err := client.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusNoContent, res.StatusCode)
	mockCatStore.AssertExpectations(t)
}

func TestHTTPHandler_DeleteCategory_NotFound(t *testing.T) {
	mockCatStore := new(MockCategoryStorer)
	server := setupTestChiServer(t, mockCatStore, nil)
	defer server.Close()

	categoryID := int64(99)
	mockCatStore.On("DeleteCategory", mock.Anything, categoryID).Return(store.ErrCategoryNotFound).Once()

	req, err := http.NewRequest(http.MethodDelete, server.URL+fmt.Sprintf("/api/v1/categories/%d", categoryID), nil)
	require.NoError(t, err)

	client := &http.Client{}
	res, err := client.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	var errResp ErrorResponse
	err = json.NewDecoder(res.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, store.ErrCategoryNotFound.Error(), errResp.Error)

	mockCatStore.AssertExpectations(t)
}
// --- End of API tests ---
