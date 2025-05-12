package store

import (
	"context"

	"product-catalog-service/internal/domain"
)

// ListCategoriesParams holds parameters for listing categories (e.g., for pagination).
type ListCategoriesParams struct {
	Limit  int
	Offset int
	// Add other filter parameters if needed in the future (e.g., ParentID)
}

// CategoryStorer defines the database operations for categories.
type CategoryStorer interface {
	CreateCategory(ctx context.Context, category *domain.Category) (*domain.Category, error)
	GetCategoryByID(ctx context.Context, id int64) (*domain.Category, error)
	ListCategories(ctx context.Context, params ListCategoriesParams) ([]domain.Category, int, error) // Returns categories and total count for pagination
	UpdateCategory(ctx context.Context, category *domain.Category) (*domain.Category, error)
	DeleteCategory(ctx context.Context, id int64) error
}

// ListProductsParams holds parameters for listing products (for pagination, filtering, sorting).
type ListProductsParams struct {
	Limit       int
	Offset      int
	SearchQuery *string // For searching by name/description
	CategoryID  *int64  // For filtering by category
	MinPrice    *float64
	MaxPrice    *float64
	IsActive    *bool   // Filter by active status
	SortBy      string  // e.g., "price", "name", "created_at"
	SortOrder   string  // "asc" or "desc"
	ProductIDs  []int64 // For fetching specific products by their IDs
}

// ProductStorer defines the database operations for products.
type ProductStorer interface {
	CreateProduct(ctx context.Context, product *domain.Product) (*domain.Product, error)
	GetProductByID(ctx context.Context, id int64) (*domain.Product, error)
	ListProducts(ctx context.Context, params ListProductsParams) ([]domain.Product, int, error) // Returns products and total count
	UpdateProduct(ctx context.Context, product *domain.Product) (*domain.Product, error)
	DeleteProduct(ctx context.Context, id int64) error
	UpdateStock(ctx context.Context, productID int64, quantityChange int32) (*domain.Product, error)
	GetRecentProducts(ctx context.Context, limit int) ([]domain.Product, error) // New method for recommendations
}
