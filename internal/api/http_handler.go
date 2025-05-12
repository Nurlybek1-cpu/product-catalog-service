package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings" // Required for string manipulation functions like ToLower

	"product-catalog-service/internal/domain"
	"product-catalog-service/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

// HTTPHandler holds dependencies for HTTP handlers.
type HTTPHandler struct {
	categoryStore store.CategoryStorer
	productStore  store.ProductStorer
	validate      *validator.Validate
}

// NewHTTPHandler creates a new HTTPHandler with dependencies.
func NewHTTPHandler(cs store.CategoryStorer, ps store.ProductStorer) *HTTPHandler {
	return &HTTPHandler{
		categoryStore: cs,
		productStore:  ps,
		validate:      validator.New(),
	}
}

// --- Helpers ---

// ErrorResponse defines the structure for JSON error responses.
type ErrorResponse struct {
	Error string `json:"error"`
	// Details interface{} `json:"details,omitempty"` // Optional: for more detailed validation errors
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, ErrorResponse{Error: message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if payload != nil { // Avoid writing empty body for 204 No Content
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			log.Printf("ERROR: Failed to encode JSON response: %v", err)
			// Fallback, as headers might have been written
			http.Error(w, `{"error": "Internal server error during JSON encoding"}`, http.StatusInternalServerError)
		}
	}
}

// --- Category Handlers ---

// CategoryCreateInput defines the expected input for creating a category.
type CategoryCreateInput struct {
	Name             string  `json:"name" validate:"required,max=255"` // Max length from DB schema
	Description      *string `json:"description" validate:"omitempty"` // No specific max, TEXT in DB
	ParentCategoryID *int64  `json:"parent_category_id" validate:"omitempty,gt=0"`
}

func (h *HTTPHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var input CategoryCreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}
	defer r.Body.Close()

	if err := h.validate.Struct(input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Validation failed: "+err.Error())
		return
	}

	category := &domain.Category{
		Name:             input.Name,
		Description:      input.Description,
		ParentCategoryID: input.ParentCategoryID,
	}

	createdCategory, err := h.categoryStore.CreateCategory(r.Context(), category)
	if err != nil {
		log.Printf("ERROR: CreateCategory store operation failed: %v", err)
		if errors.Is(err, store.ErrCategoryNameExists) {
			respondWithError(w, http.StatusConflict, store.ErrCategoryNameExists.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, "Failed to create category")
		}
		return
	}

	respondWithJSON(w, http.StatusCreated, createdCategory)
}

func (h *HTTPHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	pageStr := r.URL.Query().Get("page")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10 // Default limit
	}
	if limit > 100 { // Max limit
		limit = 100
	}

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1 // Default page
	}
	offset := (page - 1) * limit

	params := store.ListCategoriesParams{
		Limit:  limit,
		Offset: offset,
	}

	categories, totalCount, err := h.categoryStore.ListCategories(r.Context(), params)
	if err != nil {
		log.Printf("ERROR: ListCategories store operation failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve categories")
		return
	}

	totalPages := 0
	if totalCount > 0 {
		totalPages = (totalCount + limit - 1) / limit
	}
	
	// Matches OpenAPI PaginationInfo
	response := struct {
		Data       []domain.Category `json:"data"`
		Pagination struct {
			Page       int `json:"page"`
			Limit      int `json:"limit"`
			TotalItems int `json:"total_items"`
			TotalPages int `json:"total_pages"`
		} `json:"pagination"`
	}{
		Data: categories,
		Pagination: struct {
			Page       int `json:"page"`
			Limit      int `json:"limit"`
			TotalItems int `json:"total_items"`
			TotalPages int `json:"total_pages"`
		}{
			Page:       page,
			Limit:      limit,
			TotalItems: totalCount,
			TotalPages: totalPages,
		},
	}

	respondWithJSON(w, http.StatusOK, response)
}

func (h *HTTPHandler) GetCategoryByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "categoryId")
	categoryID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || categoryID <= 0 {
		respondWithError(w, http.StatusBadRequest, "Invalid category ID format")
		return
	}

	category, err := h.categoryStore.GetCategoryByID(r.Context(), categoryID)
	if err != nil {
		log.Printf("ERROR: GetCategoryByID store operation for ID %d failed: %v", categoryID, err)
		if errors.Is(err, store.ErrCategoryNotFound) {
			respondWithError(w, http.StatusNotFound, store.ErrCategoryNotFound.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, "Failed to retrieve category")
		}
		return
	}

	respondWithJSON(w, http.StatusOK, category)
}

// CategoryUpdateInput defines the expected input for updating a category.
type CategoryUpdateInput struct {
	Name             string  `json:"name" validate:"required,max=255"`
	Description      *string `json:"description" validate:"omitempty"`
	ParentCategoryID *int64  `json:"parent_category_id" validate:"omitempty,gt=0"`
}

func (h *HTTPHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "categoryId")
	categoryID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || categoryID <= 0 {
		respondWithError(w, http.StatusBadRequest, "Invalid category ID format")
		return
	}

	var input CategoryUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}
	defer r.Body.Close()

	if err := h.validate.Struct(input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Validation failed: "+err.Error())
		return
	}

	// Business rule: A category cannot be its own parent.
	if input.ParentCategoryID != nil && *input.ParentCategoryID == categoryID {
		respondWithError(w, http.StatusBadRequest, "Category cannot be its own parent")
		return
	}

	category := &domain.Category{
		ID:               categoryID,
		Name:             input.Name,
		Description:      input.Description,
		ParentCategoryID: input.ParentCategoryID,
	}

	updatedCategory, err := h.categoryStore.UpdateCategory(r.Context(), category)
	if err != nil {
		log.Printf("ERROR: UpdateCategory store operation for ID %d failed: %v", categoryID, err)
		if errors.Is(err, store.ErrCategoryNotFound) {
			respondWithError(w, http.StatusNotFound, store.ErrCategoryNotFound.Error())
		} else if errors.Is(err, store.ErrCategoryNameExists) {
			respondWithError(w, http.StatusConflict, store.ErrCategoryNameExists.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, "Failed to update category")
		}
		return
	}

	respondWithJSON(w, http.StatusOK, updatedCategory)
}

func (h *HTTPHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "categoryId")
	categoryID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || categoryID <= 0 {
		respondWithError(w, http.StatusBadRequest, "Invalid category ID format")
		return
	}

	err = h.categoryStore.DeleteCategory(r.Context(), categoryID)
	if err != nil {
		log.Printf("ERROR: DeleteCategory store operation for ID %d failed: %v", categoryID, err)
		if errors.Is(err, store.ErrCategoryNotFound) {
			respondWithError(w, http.StatusNotFound, store.ErrCategoryNotFound.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, "Failed to delete category")
		}
		return
	}

	respondWithJSON(w, http.StatusNoContent, nil) // Or w.WriteHeader(http.StatusNoContent)
}

// --- Product Handlers ---

// ProductCreateInput defines the expected input for creating a product.
type ProductCreateInput struct {
	Name          string           `json:"name" validate:"required,max=255"`
	Description   *string          `json:"description" validate:"omitempty"`
	SKU           string           `json:"sku" validate:"required,max=100"` // Max length from DB
	Price         float64          `json:"price" validate:"required,gte=0"`
	StockQuantity int32            `json:"stock_quantity" validate:"required,gte=0"` // Changed to int32
	CategoryID    *int64           `json:"category_id" validate:"omitempty,gt=0"`
	ImageURL      *string          `json:"image_url" validate:"omitempty,url,max=2048"`
	IsActive      *bool            `json:"is_active"` // Pointer to distinguish between not set and false
	Attributes    *json.RawMessage `json:"attributes,omitempty" validate:"omitempty"` // Changed to json.RawMessage
}

func (h *HTTPHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var input ProductCreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}
	defer r.Body.Close()

	if err := h.validate.Struct(input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Validation failed: "+err.Error())
		return
	}

	isActive := true // Default to true if not provided
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	product := &domain.Product{
		Name:          input.Name,
		Description:   input.Description,
		SKU:           input.SKU,
		Price:         input.Price,
		StockQuantity: input.StockQuantity,
		CategoryID:    input.CategoryID,
		ImageURL:      input.ImageURL,
		IsActive:      isActive,
		Attributes:    input.Attributes,
	}

	createdProduct, err := h.productStore.CreateProduct(r.Context(), product)
	if err != nil {
		log.Printf("ERROR: CreateProduct store operation failed: %v", err)
		if errors.Is(err, store.ErrProductSKUExists) {
			respondWithError(w, http.StatusConflict, store.ErrProductSKUExists.Error())
		} else if errors.Is(err, store.ErrCategoryNotFound) { // If category_id FK fails
			respondWithError(w, http.StatusBadRequest, "Invalid category_id: category does not exist.")
		}else {
			respondWithError(w, http.StatusInternalServerError, "Failed to create product")
		}
		return
	}

	respondWithJSON(w, http.StatusCreated, createdProduct)
}

func (h *HTTPHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	qParams := r.URL.Query()
	
	limitStr := qParams.Get("limit")
	pageStr := qParams.Get("page")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	params := store.ListProductsParams{Limit: limit, Offset: offset}

	if q := qParams.Get("q"); q != "" {
		params.SearchQuery = &q
	}
	if idStr := qParams.Get("category_id"); idStr != "" {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil && id > 0 {
			params.CategoryID = &id
		} else {
			respondWithError(w, http.StatusBadRequest, "Invalid category_id format")
			return
		}
	}
	if priceStr := qParams.Get("min_price"); priceStr != "" {
		if price, err := strconv.ParseFloat(priceStr, 64); err == nil && price >= 0 {
			params.MinPrice = &price
		} else {
			respondWithError(w, http.StatusBadRequest, "Invalid min_price format")
			return
		}
	}
	if priceStr := qParams.Get("max_price"); priceStr != "" {
		if price, err := strconv.ParseFloat(priceStr, 64); err == nil && price >= 0 {
			params.MaxPrice = &price
		} else {
			respondWithError(w, http.StatusBadRequest, "Invalid max_price format")
			return
		}
	}
	if params.MinPrice != nil && params.MaxPrice != nil && *params.MinPrice > *params.MaxPrice {
		respondWithError(w, http.StatusBadRequest, "min_price cannot exceed max_price")
		return
	}
	if activeStr := qParams.Get("is_active"); activeStr != "" {
		if b, err := strconv.ParseBool(activeStr); err == nil {
			params.IsActive = &b
		} else {
			respondWithError(w, http.StatusBadRequest, "Invalid is_active value: must be true or false")
			return
		}
	}

	params.SortBy = qParams.Get("sort_by") // Validation happens in store or can be added here
	params.SortOrder = qParams.Get("sort_order") // Validation happens in store or can be added here

	// Whitelist sort fields and order here for better API contract enforcement
	allowedSortFields := map[string]bool{"name": true, "price": true, "created_at": true, "updated_at": true, "":true} // "" for default
	if !allowedSortFields[params.SortBy] {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid sort_by field. Allowed: %v", getMapKeys(allowedSortFields)))
		return
	}
	if params.SortOrder != "" && strings.ToLower(params.SortOrder) != "asc" && strings.ToLower(params.SortOrder) != "desc" {
		respondWithError(w, http.StatusBadRequest, "Invalid sort_order value. Allowed: asc, desc")
		return
	}


	products, totalCount, err := h.productStore.ListProducts(r.Context(), params)
	if err != nil {
		log.Printf("ERROR: ListProducts store operation failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve products")
		return
	}

	totalPages := 0
	if totalCount > 0 {
		totalPages = (totalCount + limit - 1) / limit
	}
	response := struct {
		Data       []domain.Product `json:"data"`
		Pagination struct {
			Page       int `json:"page"`
			Limit      int `json:"limit"`
			TotalItems int `json:"total_items"`
			TotalPages int `json:"total_pages"`
		} `json:"pagination"`
	}{
		Data: products,
		Pagination: struct {
			Page       int `json:"page"`
			Limit      int `json:"limit"`
			TotalItems int `json:"total_items"`
			TotalPages int `json:"total_pages"`
		}{
			Page:       page,
			Limit:      limit,
			TotalItems: totalCount,
			TotalPages: totalPages,
		},
	}
	respondWithJSON(w, http.StatusOK, response)
}

// Helper to get keys from a map for error messages
func getMapKeys(m map[string]bool) []string {
    keys := make([]string, 0, len(m))
    for k := range m {
		if k != "" { // Don't list empty string default in error message
        	keys = append(keys, k)
		}
    }
    return keys
}


func (h *HTTPHandler) GetProductByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "productId")
	productID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || productID <= 0 {
		respondWithError(w, http.StatusBadRequest, "Invalid product ID format")
		return
	}

	product, err := h.productStore.GetProductByID(r.Context(), productID)
	if err != nil {
		log.Printf("ERROR: GetProductByID store operation for ID %d failed: %v", productID, err)
		if errors.Is(err, store.ErrProductNotFound) {
			respondWithError(w, http.StatusNotFound, store.ErrProductNotFound.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, "Failed to retrieve product")
		}
		return
	}
	respondWithJSON(w, http.StatusOK, product)
}

// ProductUpdateInput defines the expected input for updating a product.
type ProductUpdateInput struct {
	Name          string           `json:"name" validate:"required,max=255"`
	Description   *string          `json:"description" validate:"omitempty"`
	SKU           string           `json:"sku" validate:"required,max=100"`
	Price         float64          `json:"price" validate:"required,gte=0"`
	StockQuantity int32            `json:"stock_quantity" validate:"required,gte=0"` // Changed to int32
	CategoryID    *int64           `json:"category_id" validate:"omitempty,gt=0"`
	ImageURL      *string          `json:"image_url" validate:"omitempty,url,max=2048"`
	IsActive      *bool            `json:"is_active"`
	Attributes    *json.RawMessage `json:"attributes,omitempty" validate:"omitempty"` // Changed to json.RawMessage
}

func (h *HTTPHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "productId")
	productID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || productID <= 0 {
		respondWithError(w, http.StatusBadRequest, "Invalid product ID format")
		return
	}

	var input ProductUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}
	defer r.Body.Close()

	if err := h.validate.Struct(input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Validation failed: "+err.Error())
		return
	}

	// Get existing product to ensure it exists before update,
	// and to handle partial updates gracefully if needed (though current store.UpdateProduct updates all fields)
	// This also helps in preserving fields not included in ProductUpdateInput if the domain struct was more complex
	// For now, it mainly serves as an existence check.
	_, err = h.productStore.GetProductByID(r.Context(), productID)
	if err != nil {
		log.Printf("ERROR: Product for update (ID %d) not found: %v", productID, err)
		if errors.Is(err, store.ErrProductNotFound) {
			respondWithError(w, http.StatusNotFound, store.ErrProductNotFound.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, "Error checking product existence")
		}
		return
	}
	
	isActive := true // Default if not changing
	// If input.IsActive is explicitly provided, use its value.
	// If product.IsActive was loaded, one might default to product.IsActive
	// For simplicity, if *bool is nil, it means "don't change" or default to true if it's a create-like update.
	// Here, we take the input if provided, otherwise assume the existing one or default.
	// The current product domain model has IsActive as bool, not *bool, so we must provide a value.
	if input.IsActive != nil {
		isActive = *input.IsActive
	}


	productToUpdate := &domain.Product{
		ID:            productID,
		Name:          input.Name,
		Description:   input.Description,
		SKU:           input.SKU,
		Price:         input.Price,
		StockQuantity: input.StockQuantity,
		CategoryID:    input.CategoryID,
		ImageURL:      input.ImageURL,
		IsActive:      isActive, // Use the determined isActive value
		Attributes:    input.Attributes,
	}

	updatedProduct, err := h.productStore.UpdateProduct(r.Context(), productToUpdate)
	if err != nil {
		log.Printf("ERROR: UpdateProduct store operation for ID %d failed: %v", productID, err)
		if errors.Is(err, store.ErrProductNotFound) { // Should have been caught by GetProductByID above
			respondWithError(w, http.StatusNotFound, store.ErrProductNotFound.Error())
		} else if errors.Is(err, store.ErrProductSKUExists) {
			respondWithError(w, http.StatusConflict, store.ErrProductSKUExists.Error())
		} else if errors.Is(err, store.ErrCategoryNotFound) { // If category_id FK fails
			respondWithError(w, http.StatusBadRequest, "Invalid category_id: category does not exist.")
		} else {
			respondWithError(w, http.StatusInternalServerError, "Failed to update product")
		}
		return
	}

	respondWithJSON(w, http.StatusOK, updatedProduct)
}

func (h *HTTPHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "productId")
	productID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || productID <= 0 {
		respondWithError(w, http.StatusBadRequest, "Invalid product ID format")
		return
	}

	err = h.productStore.DeleteProduct(r.Context(), productID)
	if err != nil {
		log.Printf("ERROR: DeleteProduct store operation for ID %d failed: %v", productID, err)
		if errors.Is(err, store.ErrProductNotFound) {
			respondWithError(w, http.StatusNotFound, store.ErrProductNotFound.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, "Failed to delete product")
		}
		return
	}

	respondWithJSON(w, http.StatusNoContent, nil)
}

func (h *HTTPHandler) GetProductRecommendations(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 5 // Default limit
	}
	if limit > 20 { // Max limit for recommendations
		limit = 20
	}

	// For now, using GetRecentProducts as the recommendation strategy
	// Your OpenAPI spec also had optional product_id or user_id for recommendations,
	// which would require different store methods and more complex logic here.
	recommendations, err := h.productStore.GetRecentProducts(r.Context(), limit)
	if err != nil {
		log.Printf("ERROR: GetProductRecommendations (GetRecentProducts) failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch product recommendations")
		return
	}

	if recommendations == nil { // Ensure empty list instead of null if store returns nil slice
		recommendations = []domain.Product{}
	}

	respondWithJSON(w, http.StatusOK, recommendations)
}

// --- Route Registration ---

// RegisterRoutes sets up the HTTP routes for the service.
func (h *HTTPHandler) RegisterRoutes(r chi.Router) {
	// Could add middleware here: e.g., logging, auth, rate limiting
	// r.Use(middleware.Logger)
	// r.Use(AuthMiddleware) // Placeholder for your auth middleware

	r.Route("/api/v1/categories", func(r chi.Router) {
		r.Post("/", h.CreateCategory)      // POST /api/v1/categories
		r.Get("/", h.ListCategories)        // GET /api/v1/categories
		r.Route("/{categoryId}", func(r chi.Router) {
			r.Get("/", h.GetCategoryByID)   // GET /api/v1/categories/{categoryId}
			r.Put("/", h.UpdateCategory)    // PUT /api/v1/categories/{categoryId}
			r.Delete("/", h.DeleteCategory) // DELETE /api/v1/categories/{categoryId}
		})
	})

	r.Route("/api/v1/products", func(r chi.Router) {
		r.Post("/", h.CreateProduct)        // POST /api/v1/products
		r.Get("/", h.ListProducts)          // GET /api/v1/products
		// Ensure this is before the {productId} route to avoid "recommendations" being treated as an ID
		r.Get("/recommendations", h.GetProductRecommendations) // GET /api/v1/products/recommendations

		r.Route("/{productId}", func(r chi.Router) {
			r.Get("/", h.GetProductByID)     // GET /api/v1/products/{productId}
			r.Put("/", h.UpdateProduct)      // PUT /api/v1/products/{productId}
			r.Delete("/", h.DeleteProduct)   // DELETE /api/v1/products/{productId}
		})
	})
}