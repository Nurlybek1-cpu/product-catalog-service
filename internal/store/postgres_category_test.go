// File: product-catalog-service/internal/store/postgres_category_test.go
package store

import (
	"context"
	"database/sql"
	"errors" // For errors.Is
	"regexp" // For sqlmock query matching
	"testing"
	"time"

	"product-catalog-service/internal/domain" // Corrected import

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"                  // For pq.Error if needed for specific error simulation
	"github.com/stretchr/testify/assert" // For assertions
	"github.com/stretchr/testify/require"
)

// Helper function to create a mock DB and PostgresStore for testing
func newMockDBAndStore(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *PostgresStore) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp)) // Use regexp matcher
	require.NoError(t, err, "Failed to create sqlmock")

	// Create the store with the mock DB
	// NewPostgresStore now directly takes *sql.DB
	store := NewPostgresStore(db)
	require.NotNil(t, store, "Store should not be nil")

	return db, mock, store
}

// Helper function to get a pointer (useful for optional fields in domain structs)
func PtrTo[T any](v T) *T {
	return &v
}

func TestPostgresStore_CreateCategory(t *testing.T) {
	db, mock, store := newMockDBAndStore(t)
	defer db.Close()

	now := time.Now().Truncate(time.Millisecond) // Truncate for easier comparison
	categoryToCreate := &domain.Category{
		Name:             "Test Category",
		Description:      PtrTo("Test Description"),
		ParentCategoryID: nil, // Explicitly nil for a top-level category
	}

	expectedID := int64(1)

	// Query from store.CreateCategory
	query := regexp.QuoteMeta(`
		INSERT INTO products.categories (name, description, parent_category_id)
		VALUES ($1, $2, $3)
		RETURNING id, name, description, parent_category_id, created_at, updated_at;
	`)

	rows := sqlmock.NewRows([]string{"id", "name", "description", "parent_category_id", "created_at", "updated_at"}).
		AddRow(expectedID, categoryToCreate.Name, categoryToCreate.Description, categoryToCreate.ParentCategoryID, now, now)

	mock.ExpectQuery(query).
		WithArgs(categoryToCreate.Name, categoryToCreate.Description, categoryToCreate.ParentCategoryID).
		WillReturnRows(rows)

	createdCategory, err := store.CreateCategory(context.Background(), categoryToCreate)

	require.NoError(t, err, "CreateCategory should not return an error")
	require.NotNil(t, createdCategory, "Created category should not be nil")
	assert.Equal(t, expectedID, createdCategory.ID)
	assert.Equal(t, categoryToCreate.Name, createdCategory.Name)
	assert.Equal(t, categoryToCreate.Description, createdCategory.Description)
	assert.Equal(t, categoryToCreate.ParentCategoryID, createdCategory.ParentCategoryID)
	assert.WithinDuration(t, now, createdCategory.CreatedAt, time.Second, "CreatedAt should be close to now")
	assert.WithinDuration(t, now, createdCategory.UpdatedAt, time.Second, "UpdatedAt should be close to now")

	err = mock.ExpectationsWereMet()
	require.NoError(t, err, "SQLmock expectations were not met")
}

func TestPostgresStore_CreateCategory_NameExists(t *testing.T) {
	db, mock, store := newMockDBAndStore(t)
	defer db.Close()

	categoryToCreate := &domain.Category{
		Name:        "Existing Category",
		Description: PtrTo("Some description"),
	}

	query := regexp.QuoteMeta(`
		INSERT INTO products.categories (name, description, parent_category_id)
		VALUES ($1, $2, $3)
		RETURNING id, name, description, parent_category_id, created_at, updated_at;
	`)

	pqErr := &pq.Error{Code: "23505", Constraint: "categories_name_key"}
	mock.ExpectQuery(query).
		WithArgs(categoryToCreate.Name, categoryToCreate.Description, categoryToCreate.ParentCategoryID).
		WillReturnError(pqErr)

	createdCategory, err := store.CreateCategory(context.Background(), categoryToCreate)

	require.Error(t, err, "CreateCategory should return an error for existing name")
	assert.True(t, errors.Is(err, ErrCategoryNameExists), "Error should be ErrCategoryNameExists")
	assert.Nil(t, createdCategory, "Created category should be nil on error")

	err = mock.ExpectationsWereMet()
	require.NoError(t, err, "SQLmock expectations were not met")
}

func TestPostgresStore_GetCategoryByID_Found(t *testing.T) {
	db, mock, store := newMockDBAndStore(t)
	defer db.Close()

	categoryID := int64(1)
	now := time.Now().Truncate(time.Millisecond)
	expectedCategory := &domain.Category{
		ID:               categoryID,
		Name:             "Found Category",
		Description:      PtrTo("This is a found category"),
		ParentCategoryID: PtrTo(int64(5)),
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	query := regexp.QuoteMeta(`
		SELECT id, name, description, parent_category_id, created_at, updated_at
		FROM products.categories
		WHERE id = $1;
	`)

	rows := sqlmock.NewRows([]string{"id", "name", "description", "parent_category_id", "created_at", "updated_at"}).
		AddRow(expectedCategory.ID, expectedCategory.Name, expectedCategory.Description, expectedCategory.ParentCategoryID, expectedCategory.CreatedAt, expectedCategory.UpdatedAt)

	mock.ExpectQuery(query).WithArgs(categoryID).WillReturnRows(rows)

	category, err := store.GetCategoryByID(context.Background(), categoryID)

	require.NoError(t, err)
	require.NotNil(t, category)
	assert.Equal(t, expectedCategory.ID, category.ID)
	assert.Equal(t, expectedCategory.Name, category.Name)
	assert.Equal(t, expectedCategory.Description, category.Description)
	assert.Equal(t, expectedCategory.ParentCategoryID, category.ParentCategoryID)
	assert.Equal(t, expectedCategory.CreatedAt.Unix(), category.CreatedAt.Unix())
	assert.Equal(t, expectedCategory.UpdatedAt.Unix(), category.UpdatedAt.Unix())

	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

func TestPostgresStore_GetCategoryByID_NotFound(t *testing.T) {
	db, mock, store := newMockDBAndStore(t)
	defer db.Close()

	categoryID := int64(99)

	query := regexp.QuoteMeta(`
		SELECT id, name, description, parent_category_id, created_at, updated_at
		FROM products.categories
		WHERE id = $1;
	`)

	mock.ExpectQuery(query).WithArgs(categoryID).WillReturnError(sql.ErrNoRows)

	category, err := store.GetCategoryByID(context.Background(), categoryID)

	require.Error(t, err, "Expected an error for not found category")
	assert.True(t, errors.Is(err, ErrCategoryNotFound), "Error should be ErrCategoryNotFound")
	assert.Nil(t, category, "Category should be nil when not found")

	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

func TestPostgresStore_ListCategories(t *testing.T) {
	db, mock, store := newMockDBAndStore(t)
	defer db.Close()

	now := time.Now().Truncate(time.Millisecond)
	params := ListCategoriesParams{Limit: 2, Offset: 0}
	expectedTotalCount := 5

	// This is the query string from your store.ListCategories function, including the comment.
	// Ensure it exactly matches the one in store/postgres.go
	listQuerySQL := `
		SELECT id, name, description, parent_category_id, created_at, updated_at
		FROM products.categories
		ORDER BY name ASC -- Default sort order
		LIMIT $1 OFFSET $2;
	`
	listQuery := regexp.QuoteMeta(listQuerySQL) // Apply QuoteMeta to the exact SQL

	countQuery := regexp.QuoteMeta(`SELECT COUNT(*) FROM products.categories;`)

	listRows := sqlmock.NewRows([]string{"id", "name", "description", "parent_category_id", "created_at", "updated_at"}).
		AddRow(int64(1), "Alpha Category", PtrTo("Desc A"), nil, now, now).
		AddRow(int64(2), "Beta Category", PtrTo("Desc B"), PtrTo(int64(1)), now, now)

	countRows := sqlmock.NewRows([]string{"count"}).AddRow(expectedTotalCount)

	// Order of expectations matters if queries are distinct and ordered in the function
	mock.ExpectQuery(countQuery).WillReturnRows(countRows) // Count query first
	mock.ExpectQuery(listQuery).WithArgs(params.Limit, params.Offset).WillReturnRows(listRows)

	categories, totalCount, err := store.ListCategories(context.Background(), params)

	require.NoError(t, err)
	require.Len(t, categories, 2, "Expected 2 categories to be returned")
	assert.Equal(t, expectedTotalCount, totalCount, "Expected total count to match")
	assert.Equal(t, "Alpha Category", categories[0].Name)
	assert.Equal(t, "Beta Category", categories[1].Name)

	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

func TestPostgresStore_UpdateCategory(t *testing.T) {
	db, mock, store := newMockDBAndStore(t)
	defer db.Close()

	now := time.Now().Truncate(time.Millisecond)
	categoryToUpdate := &domain.Category{
		ID:               int64(1),
		Name:             "Updated Category Name",
		Description:      PtrTo("Updated Description"),
		ParentCategoryID: PtrTo(int64(2)),
	}

	query := regexp.QuoteMeta(`
		UPDATE products.categories
		SET name = $1, description = $2, parent_category_id = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $4
		RETURNING id, name, description, parent_category_id, created_at, updated_at;
	`)

	originalCreatedAt := now.Add(-time.Hour)
	rows := sqlmock.NewRows([]string{"id", "name", "description", "parent_category_id", "created_at", "updated_at"}).
		AddRow(categoryToUpdate.ID, categoryToUpdate.Name, categoryToUpdate.Description, categoryToUpdate.ParentCategoryID, originalCreatedAt, now)

	mock.ExpectQuery(query).
		WithArgs(categoryToUpdate.Name, categoryToUpdate.Description, categoryToUpdate.ParentCategoryID, categoryToUpdate.ID).
		WillReturnRows(rows)

	updatedCategory, err := store.UpdateCategory(context.Background(), categoryToUpdate)

	require.NoError(t, err)
	require.NotNil(t, updatedCategory)
	assert.Equal(t, categoryToUpdate.ID, updatedCategory.ID)
	assert.Equal(t, categoryToUpdate.Name, updatedCategory.Name)
	assert.Equal(t, categoryToUpdate.Description, updatedCategory.Description)
	assert.Equal(t, categoryToUpdate.ParentCategoryID, updatedCategory.ParentCategoryID)
	assert.Equal(t, originalCreatedAt.Unix(), updatedCategory.CreatedAt.Unix())
	assert.WithinDuration(t, now, updatedCategory.UpdatedAt, time.Second)

	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

func TestPostgresStore_UpdateCategory_NotFound(t *testing.T) {
	db, mock, store := newMockDBAndStore(t)
	defer db.Close()

	categoryToUpdate := &domain.Category{
		ID:               int64(99),
		Name:             "Non Existent",
		Description:      PtrTo("Desc"), // Add other fields expected by the query args
		ParentCategoryID: nil,
	}
	query := regexp.QuoteMeta(`
		UPDATE products.categories
		SET name = $1, description = $2, parent_category_id = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $4
		RETURNING id, name, description, parent_category_id, created_at, updated_at;
	`)
	mock.ExpectQuery(query).
		WithArgs(categoryToUpdate.Name, categoryToUpdate.Description, categoryToUpdate.ParentCategoryID, categoryToUpdate.ID).
		WillReturnError(sql.ErrNoRows)

	_, err := store.UpdateCategory(context.Background(), categoryToUpdate)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCategoryNotFound), "Error should be ErrCategoryNotFound")

	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

func TestPostgresStore_DeleteCategory_Success(t *testing.T) {
	db, mock, store := newMockDBAndStore(t)
	defer db.Close()

	categoryID := int64(1)
	query := regexp.QuoteMeta(`DELETE FROM products.categories WHERE id = $1;`)

	mock.ExpectExec(query).WithArgs(categoryID).WillReturnResult(sqlmock.NewResult(0, 1))

	err := store.DeleteCategory(context.Background(), categoryID)

	require.NoError(t, err, "DeleteCategory should not return an error on success")

	err = mock.ExpectationsWereMet()
	require.NoError(t, err, "SQLmock expectations were not met")
}

func TestPostgresStore_DeleteCategory_NotFound(t *testing.T) {
	db, mock, store := newMockDBAndStore(t)
	defer db.Close()

	categoryID := int64(99)
	query := regexp.QuoteMeta(`DELETE FROM products.categories WHERE id = $1;`)

	mock.ExpectExec(query).WithArgs(categoryID).WillReturnResult(sqlmock.NewResult(0, 0))

	err := store.DeleteCategory(context.Background(), categoryID)

	require.Error(t, err, "DeleteCategory should return an error if no rows were affected")
	assert.True(t, errors.Is(err, ErrCategoryNotFound), "Error should be ErrCategoryNotFound")

	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

// --- End of store tests ---
