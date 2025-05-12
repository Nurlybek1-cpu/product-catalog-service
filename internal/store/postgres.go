package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"log"

	"github.com/lib/pq"

	"product-catalog-service/internal/domain"
)

// Predefined errors for store operations
var (
	ErrCategoryNotFound   = errors.New("store: category not found")
	ErrCategoryNameExists = errors.New("store: category name already exists")
	ErrProductNotFound    = errors.New("store: product not found")
	ErrProductSKUExists   = errors.New("store: product SKU already exists")
	ErrInsufficientStock  = errors.New("store: insufficient stock or update constraint violation")
	ErrUpdateFailed       = errors.New("store: update failed, 0 rows affected")
)

// PostgresStore implements the CategoryStorer and ProductStorer interfaces using PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgresStore instance.
func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// --- CategoryStorer Implementation ---

func (s *PostgresStore) CreateCategory(ctx context.Context, category *domain.Category) (*domain.Category, error) {
	query := `
		INSERT INTO products.categories (name, description, parent_category_id)
		VALUES ($1, $2, $3)
		RETURNING id, name, description, parent_category_id, created_at, updated_at;
	`
	row := s.db.QueryRowContext(ctx, query, category.Name, category.Description, category.ParentCategoryID)

	var createdCategory domain.Category
	err := row.Scan(
		&createdCategory.ID,
		&createdCategory.Name,
		&createdCategory.Description,
		&createdCategory.ParentCategoryID,
		&createdCategory.CreatedAt,
		&createdCategory.UpdatedAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" { // Unique violation
			// Assuming the unique constraint is on 'name' for categories
			if strings.Contains(pqErr.Constraint, "categories_name_key") || strings.Contains(pqErr.Detail, "Key (name)") {
				return nil, ErrCategoryNameExists
			}
		}
		return nil, fmt.Errorf("store: CreateCategory failed to scan row: %w", err)
	}
	return &createdCategory, nil
}

// ListCategories retrieves a paginated list of categories.
// Note: Filtering capabilities (e.g., by parent_category_id) would require dynamic query building
// similar to ListProducts if ListCategoriesParams is extended.
func (s *PostgresStore) ListCategories(ctx context.Context, params ListCategoriesParams) ([]domain.Category, int, error) {
	countQuery := `SELECT COUNT(*) FROM products.categories;` // Simple count, no filters yet in ListCategoriesParams
	var totalCount int
	if err := s.db.QueryRowContext(ctx, countQuery).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("store: ListCategories failed to count categories: %w", err)
	}

	if totalCount == 0 {
		return []domain.Category{}, 0, nil
	}

	query := `
		SELECT id, name, description, parent_category_id, created_at, updated_at
		FROM products.categories
		ORDER BY name ASC -- Default sort order
		LIMIT $1 OFFSET $2;
	`
	rows, err := s.db.QueryContext(ctx, query, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("store: ListCategories failed to query categories: %w", err)
	}
	defer rows.Close()

	categories := make([]domain.Category, 0, params.Limit)
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.ParentCategoryID, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("store: ListCategories failed to scan category row: %w", err)
		}
		categories = append(categories, c)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("store: ListCategories iteration error: %w", err)
	}

	return categories, totalCount, nil
}

func (s *PostgresStore) GetCategoryByID(ctx context.Context, id int64) (*domain.Category, error) {
	query := `
		SELECT id, name, description, parent_category_id, created_at, updated_at
		FROM products.categories
		WHERE id = $1;
	`
	var category domain.Category
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&category.ID,
		&category.Name,
		&category.Description,
		&category.ParentCategoryID,
		&category.CreatedAt,
		&category.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("store: GetCategoryByID failed to scan row: %w", err)
	}
	return &category, nil
}

func (s *PostgresStore) UpdateCategory(ctx context.Context, category *domain.Category) (*domain.Category, error) {
	query := `
		UPDATE products.categories
		SET name = $1, description = $2, parent_category_id = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $4
		RETURNING id, name, description, parent_category_id, created_at, updated_at;
	`
	var updatedCategory domain.Category
	err := s.db.QueryRowContext(ctx, query, category.Name, category.Description, category.ParentCategoryID, category.ID).Scan(
		&updatedCategory.ID,
		&updatedCategory.Name,
		&updatedCategory.Description,
		&updatedCategory.ParentCategoryID,
		&updatedCategory.CreatedAt,
		&updatedCategory.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCategoryNotFound // Or ErrUpdateFailed if ID existed but was concurrently deleted
		}
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			if strings.Contains(pqErr.Constraint, "categories_name_key") || strings.Contains(pqErr.Detail, "Key (name)"){
				return nil, ErrCategoryNameExists
			}
		}
		return nil, fmt.Errorf("store: UpdateCategory failed to scan row: %w", err)
	}
	return &updatedCategory, nil
}

func (s *PostgresStore) DeleteCategory(ctx context.Context, id int64) error {
	query := `DELETE FROM products.categories WHERE id = $1;`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("store: DeleteCategory failed to execute delete: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		// This error is less common for RowsAffected after a successful Exec, but good to check
		return fmt.Errorf("store: DeleteCategory failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

// --- ProductStorer Implementation ---

func (s *PostgresStore) CreateProduct(ctx context.Context, product *domain.Product) (*domain.Product, error) {
	query := `
		INSERT INTO products.products 
			(name, description, sku, price, stock_quantity, category_id, image_url, is_active, attributes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, name, description, sku, price, stock_quantity, category_id, image_url, is_active, attributes, created_at, updated_at;
	`
	var attributesJSON []byte // For handling nullable JSONB
	if product.Attributes != nil && len(*product.Attributes) > 0 {
		attributesJSON = *product.Attributes
	} else {
        attributesJSON = []byte("null") // Or []byte("{}") if you prefer empty object over SQL NULL
    }


	row := s.db.QueryRowContext(ctx, query,
		product.Name, product.Description, product.SKU, product.Price, product.StockQuantity,
		product.CategoryID, product.ImageURL, product.IsActive, attributesJSON,
	)

	var createdProduct domain.Product
	var scannedAttributes sql.NullString // Use sql.NullString for attributes to handle SQL NULL properly

	err := row.Scan(
		&createdProduct.ID, &createdProduct.Name, &createdProduct.Description, &createdProduct.SKU,
		&createdProduct.Price, &createdProduct.StockQuantity, &createdProduct.CategoryID, &createdProduct.ImageURL,
		&createdProduct.IsActive, &scannedAttributes,
		&createdProduct.CreatedAt, &createdProduct.UpdatedAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" { // Unique violation
			// Assuming the unique constraint is on 'sku' for products
			if strings.Contains(pqErr.Constraint, "products_sku_key") || strings.Contains(pqErr.Detail, "Key (sku)"){
				return nil, ErrProductSKUExists
			}
		}
		return nil, fmt.Errorf("store: CreateProduct failed to scan row: %w", err)
	}

	if scannedAttributes.Valid && scannedAttributes.String != "" && scannedAttributes.String != "null" {
		rawMsg := json.RawMessage(scannedAttributes.String)
		createdProduct.Attributes = &rawMsg
	}

	return &createdProduct, nil
}

func (s *PostgresStore) ListProducts(ctx context.Context, params ListProductsParams) ([]domain.Product, int, error) {
	var queryArgs []interface{}
	var whereClauses []string
	argID := 1

	if params.SearchQuery != nil && *params.SearchQuery != "" {
		// Search in name OR description
		whereClauses = append(whereClauses, fmt.Sprintf("(name ILIKE $%d OR description ILIKE $%d)", argID, argID+1))
		searchTerm := "%" + *params.SearchQuery + "%"
		queryArgs = append(queryArgs, searchTerm, searchTerm)
		argID += 2
	}
	if params.CategoryID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("category_id = $%d", argID))
		queryArgs = append(queryArgs, *params.CategoryID)
		argID++
	}
	if params.MinPrice != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("price >= $%d", argID))
		queryArgs = append(queryArgs, *params.MinPrice)
		argID++
	}
	if params.MaxPrice != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("price <= $%d", argID))
		queryArgs = append(queryArgs, *params.MaxPrice)
		argID++
	}
	if params.IsActive != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("is_active = $%d", argID))
		queryArgs = append(queryArgs, *params.IsActive)
		argID++
	}
	if len(params.ProductIDs) > 0 {
		placeholders := make([]string, len(params.ProductIDs))
		for i, pid := range params.ProductIDs {
			placeholders[i] = fmt.Sprintf("$%d", argID+i)
			queryArgs = append(queryArgs, pid)
		}
		whereClauses = append(whereClauses, fmt.Sprintf("id IN (%s)", strings.Join(placeholders, ",")))
		argID += len(params.ProductIDs)
	}

	whereCondition := ""
	if len(whereClauses) > 0 {
		whereCondition = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := "SELECT COUNT(*) FROM products.products" + whereCondition
	var totalCount int
	if err := s.db.QueryRowContext(ctx, countQuery, queryArgs...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("store: ListProducts failed to count products: %w", err)
	}

	if totalCount == 0 {
		return []domain.Product{}, 0, nil
	}
	
	sortColumn := "created_at" // Default sort
	allowedSortColumns := map[string]string{
		"name":       "name",
		"price":      "price",
		"created_at": "created_at",
		"updated_at": "updated_at",
	}
	if col, ok := allowedSortColumns[strings.ToLower(params.SortBy)]; ok {
		sortColumn = col
	}

	sortOrder := "ASC" // Default order
	if strings.ToUpper(params.SortOrder) == "DESC" {
		sortOrder = "DESC"
	}

	dataQueryPreamble := `
		SELECT id, name, description, sku, price, stock_quantity, category_id, image_url, is_active, attributes, created_at, updated_at
		FROM products.products
	`
	dataQuery := fmt.Sprintf("%s%s ORDER BY %s %s LIMIT $%d OFFSET $%d",
		dataQueryPreamble, whereCondition, sortColumn, sortOrder, argID, argID+1)
	
	finalQueryArgs := append(queryArgs, params.Limit, params.Offset)

	rows, err := s.db.QueryContext(ctx, dataQuery, finalQueryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("store: ListProducts failed to query products: %w", err)
	}
	defer rows.Close()

	products := make([]domain.Product, 0, params.Limit)
	for rows.Next() {
		var p domain.Product
		var scannedAttributes sql.NullString
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Description, &p.SKU, &p.Price, &p.StockQuantity,
			&p.CategoryID, &p.ImageURL, &p.IsActive, &scannedAttributes,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("store: ListProducts failed to scan product row: %w", err)
		}
		if scannedAttributes.Valid && scannedAttributes.String != "" && scannedAttributes.String != "null" {
			rawMsg := json.RawMessage(scannedAttributes.String)
			p.Attributes = &rawMsg
		}
		products = append(products, p)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("store: ListProducts iteration error: %w", err)
	}

	return products, totalCount, nil
}

func (s *PostgresStore) GetProductByID(ctx context.Context, id int64) (*domain.Product, error) {
	query := `
		SELECT id, name, description, sku, price, stock_quantity, category_id, image_url, is_active, attributes, created_at, updated_at
		FROM products.products
		WHERE id = $1;
	`
	var product domain.Product
	var scannedAttributes sql.NullString
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&product.ID, &product.Name, &product.Description, &product.SKU, &product.Price, &product.StockQuantity,
		&product.CategoryID, &product.ImageURL, &product.IsActive, &scannedAttributes,
		&product.CreatedAt, &product.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("store: GetProductByID failed to scan row: %w", err)
	}

	if scannedAttributes.Valid && scannedAttributes.String != "" && scannedAttributes.String != "null" {
		rawMsg := json.RawMessage(scannedAttributes.String)
		product.Attributes = &rawMsg
	}
	return &product, nil
}

func (s *PostgresStore) UpdateProduct(ctx context.Context, product *domain.Product) (*domain.Product, error) {
	query := `
		UPDATE products.products
		SET name = $1, description = $2, sku = $3, price = $4, stock_quantity = $5, 
			category_id = $6, image_url = $7, is_active = $8, attributes = $9, updated_at = CURRENT_TIMESTAMP
		WHERE id = $10
		RETURNING id, name, description, sku, price, stock_quantity, category_id, image_url, is_active, attributes, created_at, updated_at;
	`
	var attributesJSON []byte
	if product.Attributes != nil && len(*product.Attributes) > 0 {
		attributesJSON = *product.Attributes
	} else {
        attributesJSON = []byte("null") // Or []byte("{}")
    }


	var updatedProduct domain.Product
	var scannedAttributes sql.NullString
	err := s.db.QueryRowContext(ctx, query,
		product.Name, product.Description, product.SKU, product.Price, product.StockQuantity,
		product.CategoryID, product.ImageURL, product.IsActive, attributesJSON, product.ID,
	).Scan(
		&updatedProduct.ID, &updatedProduct.Name, &updatedProduct.Description, &updatedProduct.SKU,
		&updatedProduct.Price, &updatedProduct.StockQuantity, &updatedProduct.CategoryID, &updatedProduct.ImageURL,
		&updatedProduct.IsActive, &scannedAttributes,
		&updatedProduct.CreatedAt, &updatedProduct.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Could be that the product ID does not exist.
			return nil, ErrProductNotFound
		}
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" { // Unique violation on SKU, for example
			if strings.Contains(pqErr.Constraint, "products_sku_key") || strings.Contains(pqErr.Detail, "Key (sku)"){
				return nil, ErrProductSKUExists
			}
		}
		return nil, fmt.Errorf("store: UpdateProduct failed to scan row: %w", err)
	}

	if scannedAttributes.Valid && scannedAttributes.String != "" && scannedAttributes.String != "null" {
		rawMsg := json.RawMessage(scannedAttributes.String)
		updatedProduct.Attributes = &rawMsg
	}
	return &updatedProduct, nil
}

func (s *PostgresStore) DeleteProduct(ctx context.Context, id int64) error {
	query := `DELETE FROM products.products WHERE id = $1;`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("store: DeleteProduct failed to execute delete: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: DeleteProduct failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrProductNotFound
	}
	return nil
}

func (s *PostgresStore) UpdateStock(ctx context.Context, productID int64, quantityChange int32) (*domain.Product, error) {
	// This query attempts to update stock and ensures it doesn't go below zero.
	// The "AND stock_quantity + $1 >= 0" clause acts as a precondition.
	// If it fails (e.g. product not found, or stock would become negative), ErrNoRows is returned by QueryRowContext.
	query := `
		UPDATE products.products
		SET stock_quantity = stock_quantity + $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND stock_quantity + $1 >= 0 
		RETURNING id, name, description, sku, price, stock_quantity, category_id, image_url, is_active, attributes, created_at, updated_at;
	`
	var updatedProduct domain.Product
	var scannedAttributes sql.NullString

	err := s.db.QueryRowContext(ctx, query, quantityChange, productID).Scan(
		&updatedProduct.ID, &updatedProduct.Name, &updatedProduct.Description, &updatedProduct.SKU,
		&updatedProduct.Price, &updatedProduct.StockQuantity, &updatedProduct.CategoryID, &updatedProduct.ImageURL,
		&updatedProduct.IsActive, &scannedAttributes,
		&updatedProduct.CreatedAt, &updatedProduct.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// This error means either the product was not found, or the stock update would violate a constraint (e.g., go negative).
			// We might need to check if the product exists separately to return a more specific error.
			// For now, we'll check existence first to provide a clearer error.
			var exists bool
			checkExistenceQuery := "SELECT EXISTS(SELECT 1 FROM products.products WHERE id = $1)"
			s.db.QueryRowContext(ctx, checkExistenceQuery, productID).Scan(&exists)
			if !exists {
				return nil, ErrProductNotFound
			}
			return nil, ErrInsufficientStock // Product exists, so stock update condition failed
		}
		return nil, fmt.Errorf("store: UpdateStock failed to scan row: %w", err)
	}

	if scannedAttributes.Valid && scannedAttributes.String != "" && scannedAttributes.String != "null" {
		rawMsg := json.RawMessage(scannedAttributes.String)
		updatedProduct.Attributes = &rawMsg
	}
	return &updatedProduct, nil
}

func (s *PostgresStore) GetRecentProducts(ctx context.Context, limit int) ([]domain.Product, error) {
	if limit <= 0 { // Basic validation for limit
		return []domain.Product{}, nil
	}
	query := `
		SELECT id, name, description, sku, price, stock_quantity, category_id, image_url, is_active, attributes, created_at, updated_at
		FROM products.products
		WHERE is_active = TRUE
		ORDER BY created_at DESC
		LIMIT $1;
	`
	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("store: GetRecentProducts failed to query products: %w", err)
	}
	defer rows.Close()

	// Pre-allocate slice with capacity if limit is reasonable
	products := make([]domain.Product, 0, limit) 
	for rows.Next() {
		var p domain.Product
		var scannedAttributes sql.NullString
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Description, &p.SKU, &p.Price, &p.StockQuantity,
			&p.CategoryID, &p.ImageURL, &p.IsActive, &scannedAttributes,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("store: GetRecentProducts failed to scan product row: %w", err)
		}
		if scannedAttributes.Valid && scannedAttributes.String != "" && scannedAttributes.String != "null" {
			rawMsg := json.RawMessage(scannedAttributes.String)
			p.Attributes = &rawMsg
		}
		products = append(products, p)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("store: GetRecentProducts iteration error: %w", err)
	}
	return products, nil
}

func (s *PostgresStore) Close() error {
	if s.db != nil {
		log.Println("INFO: Closing database connection pool...")
		err := s.db.Close()
		if err != nil {
			log.Printf("ERROR: Failed to close database connection pool: %v", err)
			return err
		}
		log.Println("INFO: Database connection pool closed successfully.")
		return nil
	}
	return nil
}