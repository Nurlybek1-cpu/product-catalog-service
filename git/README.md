# Product Catalog Service 🚀

## Overview

The **Product Catalog Service** is a core microservice for an e-commerce platform. It manages all aspects of products and categories, including details, inventory, search, filtering, and recommendations. This service exposes both HTTP/REST and gRPC APIs for interaction.

---

## ✨ Features

* **Product Management**: CRUD operations for products (name, description, SKU, price, images, attributes).
* **Category Management**: CRUD operations for categories, with support for hierarchical structures.
* **Inventory Management**: Real-time stock quantity tracking.
* **Listing & Pagination**: Efficient listing of products and categories with pagination.
* **Search & Filtering**:

  * Search products by name and description.
  * Filter by category, price range, and active status.
  * Sort listings by attributes (name, price, creation date).
* **Recommendations**: Simple product recommendation features (e.g., recently added).
* **Stock Availability**: gRPC endpoint for other services (like Order Service) to check product availability and current price.
* **Dual APIs**:

  * **HTTP/REST API**: Client-facing interactions.
  * **gRPC API**: Efficient internal service-to-service communication.

---

## 🛠️ Technologies Used

* **Language**: Go (Golang)
* **Database**: PostgreSQL
* **HTTP Router**: [Chi](https://github.com/go-chi/chi/v5)
* **gRPC**: `google.golang.org/grpc`
* **Protobuf**: Protocol Buffers for gRPC service definitions
* **Configuration**: [`envconfig`](https://github.com/kelseyhightower/envconfig)
* **Validation**: [`go-playground/validator/v10`](https://github.com/go-playground/validator)
* **Database Driver**: [`github.com/lib/pq`](https://github.com/lib/pq)
* **Testing**: Go's built-in testing, [`testify`](https://github.com/stretchr/testify), [`sqlmock`](https://github.com/DATA-DOG/go-sqlmock)
* **Containerization**: Docker

---

## 📋 Prerequisites

* Go (version 1.22 or as specified in `go.mod`)
* PostgreSQL Server (running and accessible)
* Docker (optional, for containerized runs)
* `protoc` compiler and Go gRPC plugins (for regenerating Protobuf code)
* `grpcurl` (optional, for testing gRPC endpoints)

---

## ⚙️ Configuration

The service is configured via environment variables. See `internal/config/config.go` for all options.

* `APP_ENV`: Application environment (e.g., `development`, `production`). Default: `development`.
* `LOG_LEVEL`: Logging level (e.g., `info`, `debug`). Default: `info`.
* `HTTP_SERVER_PORT`: Port for the HTTP server. Default: `8081`.
* `GRPC_SERVER_PORT`: Port for the gRPC server. Default: `9090`.
* **PostgreSQL Settings**:

  * `POSTGRES_HOST` (required)
  * `POSTGRES_PORT` (default: `5432`)
  * `POSTGRES_USER` (required)
  * `POSTGRES_PASSWORD` (required)
  * `POSTGRES_DBNAME` (required)
  * `POSTGRES_SSLMODE` (default: `disable`)

---

## 🚀 Running Locally

1. **Clone the repository**:

   ```bash
   git clone <repository_url>
   cd product-catalog-service
   ```

2. **Ensure PostgreSQL is running** and create the database specified in `POSTGRES_DBNAME`.

3. **Create schema or run migrations** (e.g., see `docs/database_schema.md`).

4. **Set environment variables** (or use a `.env` file with `godotenv`):

   ```powershell
   $env:POSTGRES_HOST="your_db_host"
   $env:POSTGRES_USER="your_db_user"
   $env:POSTGRES_PASSWORD="your_db_password"
   $env:POSTGRES_DBNAME="your_db_name"
   # ...other variables as needed
   ```

5. **Install dependencies**:

   ```bash
   go mod tidy
   go mod download
   ```

6. **Build the application**:

   ```bash
   go build -o product-catalog-service ./cmd/main.go
   ```

7. **Run the application**:

   ```bash
   ./product-catalog-service
   ```

Logs will indicate the service is listening on the configured HTTP and gRPC ports.

---

## 🌐 API Endpoints

### HTTP Endpoints (Base URL: `/api/v1`)

#### Categories

* `POST /categories` : Create a new category.
* `GET /categories` : List categories (with pagination).
* `GET /categories/{categoryId}` : Get details of a category.
* `PUT /categories/{categoryId}` : Update a category.
* `DELETE /categories/{categoryId}` : Delete a category.

#### Products

* `POST /products` : Create a new product.
* `GET /products` : List products (pagination, search, filter, sort).
* `GET /products/{productId}` : Get details of a product.
* `PUT /products/{productId}` : Update a product.
* `DELETE /products/{productId}` : Delete a product.
* `GET /products/recommendations` : Get product recommendations.

#### Health Check

* `GET /healthz` : Service health status.

Refer to `api/openapi.yaml` for full REST documentation.

### gRPC Service (Definition: `product.v1.ProductCatalogService`)

* `GetCategoryDetails`
* `ListCategories`
* `InternalListProducts`
* `GetProductDetails`
* `ListProducts`
* `InternalUpdateStock`
* `CheckProductsAvailability`

See `proto/v1/product/product.proto` and `proto/v1/common/common.proto`.

---

## 🐳 Running with Docker

1. **Build Docker image**:

   ```bash
   docker build -t product-catalog-service:latest .
   ```

2. **Run Docker container** (pass required environment variables):

   ```bash
   docker run -d -p 8081:8081 -p 9090:9090 \
     -e APP_ENV="docker" \
     -e HTTP_SERVER_PORT="8081" \
     -e GRPC_SERVER_PORT="9090" \
     -e POSTGRES_HOST="host.docker.internal" \  # For Docker Desktop
     -e POSTGRES_USER="your_user" \
     -e POSTGRES_PASSWORD="your_password" \
     -e POSTGRES_DBNAME="your_dbname" \
     --name product-catalog-service \
     product-catalog-service:latest
   ```

3. **Check container logs**:

   ```bash
   docker logs product-catalog-service
   ```

---

## 🧪 Running Tests

* **All tests**:

  ```bash
  go test ./...
  ```

* **Specific package** (e.g., `store`):

  ```bash
  go test ./internal/store
  ```

---

*Happy coding!*
