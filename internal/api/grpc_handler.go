package api

import (
	"context"
	"encoding/json" // For converting domain.Product.Attributes
	"errors"
	"log"
	"strconv" // For basic pagination token example

	"product-catalog-service/internal/domain"
	"product-catalog-service/internal/store"

	commonpb "product-catalog-service/proto/v1/common"
	productpb "product-catalog-service/proto/v1/product"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb" // For product attributes
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCHandler implements the gRPC server for the ProductCatalogService.
type GRPCHandler struct {
	productpb.UnimplementedProductCatalogServiceServer // Essential for forward compatibility

	categoryStore store.CategoryStorer
	productStore  store.ProductStorer
}

// NewGRPCHandler creates a new GRPCHandler.
func NewGRPCHandler(cs store.CategoryStorer, ps store.ProductStorer) *GRPCHandler {
	return &GRPCHandler{
		categoryStore: cs,
		productStore:  ps,
	}
}

// --- Helper: Error Mapping ---
func mapStoreErrorToGrpcStatus(err error, resourceName string, resourceID interface{}) error {
	if err == nil {
		return nil
	}
	log.Printf("ERROR: Store operation for %s ID %v failed: %v", resourceName, resourceID, err)

	switch {
	case errors.Is(err, store.ErrCategoryNotFound), errors.Is(err, store.ErrProductNotFound):
		return status.Errorf(codes.NotFound, "%s with ID %v not found", resourceName, resourceID)
	case errors.Is(err, store.ErrCategoryNameExists):
		return status.Errorf(codes.AlreadyExists, "A %s with the given name already exists", resourceName)
	case errors.Is(err, store.ErrProductSKUExists):
		return status.Errorf(codes.AlreadyExists, "A %s with the given SKU already exists", resourceName)
	case errors.Is(err, store.ErrInsufficientStock):
		return status.Errorf(codes.FailedPrecondition, "Insufficient stock for %s ID %v, or operation violates constraints", resourceName, resourceID)
	default:
		return status.Errorf(codes.Internal, "Failed to process request for %s ID %v: %v", resourceName, resourceID, err)
	}
}

// --- Category gRPC Methods Implementation ---

func (s *GRPCHandler) GetCategoryDetails(ctx context.Context, req *productpb.GetCategoryDetailsRequest) (*productpb.GetCategoryDetailsResponse, error) {
	categoryID := req.GetCategoryId()
	log.Printf("INFO: Received gRPC GetCategoryDetails request for ID: %d", categoryID)

	if categoryID <= 0 {
		log.Printf("WARN: Invalid Category ID received: %d", categoryID)
		return nil, status.Errorf(codes.InvalidArgument, "Category ID must be a positive integer")
	}

	domainCategory, err := s.categoryStore.GetCategoryByID(ctx, categoryID)
	if err != nil {
		return nil, mapStoreErrorToGrpcStatus(err, "Category", categoryID)
	}

	log.Printf("INFO: Successfully fetched category ID %d", categoryID)
	return &productpb.GetCategoryDetailsResponse{
		Category: convertDomainCategoryToProto(domainCategory),
	}, nil
}

func (s *GRPCHandler) ListCategoriesInternal(ctx context.Context, req *productpb.ListCategoriesInternalRequest) (*productpb.ListCategoriesInternalResponse, error) {
	parentCatID := req.GetParentCategoryId() // Optional parent category ID for filtering
	log.Printf("INFO: Received gRPC ListCategoriesInternal request. PageSize: %d, PageToken: '%s', ParentCategoryID: %d (0 if not set)",
		req.GetPageInfo().GetPageSize(), req.GetPageInfo().GetPageToken(), parentCatID)

	limit32 := req.GetPageInfo().GetPageSize()
	if limit32 <= 0 {
		limit32 = 10 // Default page size
	}
	if limit32 > 100 { // Max page size
		limit32 = 100
	}
	limit := int(limit32)

	offset := 0
	if req.GetPageInfo().GetPageToken() != "" {
		parsedOffset, err := strconv.Atoi(req.GetPageInfo().GetPageToken())
		if err == nil && parsedOffset >= 0 { // Allow offset 0
			offset = parsedOffset
		} else if err != nil {
			log.Printf("WARN: Could not parse page_token '%s' as offset: %v. Defaulting to offset 0.", req.GetPageInfo().GetPageToken(), err)
		}
	}

	storeParams := store.ListCategoriesParams{
		Limit:  limit,
		Offset: offset,
		// Note: To filter by parent_category_id, ListCategoriesParams in store/interfaces.go
		// and its implementation in store/postgres.go would need to support it.
		// Example: if parentCatID > 0 { storeParams.ParentID = &parentCatID }
	}

	domainCategories, totalCount, err := s.categoryStore.ListCategories(ctx, storeParams)
	if err != nil {
		log.Printf("ERROR: Error listing categories from store: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to list categories: %v", err)
	}

	protoCategories := make([]*productpb.Category, len(domainCategories))
	for i := range domainCategories {
		protoCategories[i] = convertDomainCategoryToProto(&domainCategories[i])
	}

	var nextPageToken string
	if offset+len(protoCategories) < totalCount {
		nextPageToken = strconv.Itoa(offset + len(protoCategories))
	}

	log.Printf("INFO: Returning %d categories, total available: %d, next page token: '%s'", len(protoCategories), totalCount, nextPageToken)
	return &productpb.ListCategoriesInternalResponse{
		Categories: protoCategories,
		PageInfo: &commonpb.PageInfoResponse{
			NextPageToken: nextPageToken,
			TotalSize:     int32(totalCount),
		},
	}, nil
}

// --- Product gRPC Methods Implementation ---

func (s *GRPCHandler) GetProductDetails(ctx context.Context, req *productpb.GetProductDetailsRequest) (*productpb.GetProductDetailsResponse, error) {
	productID := req.GetProductId()
	log.Printf("INFO: Received gRPC GetProductDetails request for ID: %d", productID)

	if productID <= 0 {
		log.Printf("WARN: Invalid Product ID received: %d", productID)
		return nil, status.Errorf(codes.InvalidArgument, "Product ID must be a positive integer")
	}

	domainProduct, err := s.productStore.GetProductByID(ctx, productID)
	if err != nil {
		return nil, mapStoreErrorToGrpcStatus(err, "Product", productID)
	}

	protoProduct, err := convertDomainProductToProto(domainProduct)
	if err != nil {
		log.Printf("ERROR: Failed to convert domain product to proto for ID %d: %v", productID, err)
		return nil, status.Errorf(codes.Internal, "Failed to process product data for ID %d", productID)
	}

	log.Printf("INFO: Successfully fetched product ID %d", productID)
	return &productpb.GetProductDetailsResponse{
		Product: protoProduct,
	}, nil
}

func (s *GRPCHandler) ListProductsInternal(ctx context.Context, req *productpb.ListProductsInternalRequest) (*productpb.ListProductsInternalResponse, error) {
	log.Printf("INFO: Received gRPC ListProductsInternal request. PageSize: %d, PageToken: '%s', CategoryID: %d, ProductIDs: %v, IncludeInactive: %t",
		req.GetPageInfo().GetPageSize(), req.GetPageInfo().GetPageToken(), req.GetCategoryId(), req.GetProductIds(), req.GetIncludeInactive())

	limit32 := req.GetPageInfo().GetPageSize()
	if limit32 <= 0 {
		limit32 = 10
	}
	if limit32 > 100 {
		limit32 = 100
	}
	limit := int(limit32)

	offset := 0
	if req.GetPageInfo().GetPageToken() != "" {
		parsedOffset, err := strconv.Atoi(req.GetPageInfo().GetPageToken())
		if err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		} else if err != nil {
			log.Printf("WARN: Could not parse page_token '%s' as offset: %v. Defaulting to offset 0.", req.GetPageInfo().GetPageToken(), err)
		}
	}

	storeParams := store.ListProductsParams{
		Limit:      limit,
		Offset:     offset,
		ProductIDs: req.GetProductIds(), // Pass through if store supports it
	}
	if req.GetCategoryId() > 0 {
		catID := req.GetCategoryId()
		storeParams.CategoryID = &catID
	}
	if req.GetIncludeInactive() { // If true, we want to fetch all; if false or not set, filter by active (store default or explicit)
		// The store.ListProductsParams.IsActive is *bool.
		// If IncludeInactive is false (default), we might want IsActive = true.
		// If IncludeInactive is true, we don't set IsActive filter (meaning fetch all, active or inactive).
		if !req.GetIncludeInactive() {
			isActiveTrue := true
			storeParams.IsActive = &isActiveTrue // Default to fetching only active products
		}
		// If req.GetIncludeInactive() is true, storeParams.IsActive remains nil, so the store won't filter by active status.
	} else {
		// Default behavior if IncludeInactive is not specified or false: fetch active products
		isActiveTrue := true
		storeParams.IsActive = &isActiveTrue
	}


	domainProducts, totalCount, err := s.productStore.ListProducts(ctx, storeParams)
	if err != nil {
		log.Printf("ERROR: Error listing products from store: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to list products: %v", err)
	}

	protoProducts := make([]*productpb.Product, len(domainProducts))
	for i := range domainProducts {
		convertedProduct, convErr := convertDomainProductToProto(&domainProducts[i])
		if convErr != nil {
			log.Printf("ERROR: Failed to convert domain product to proto during ListProductsInternal for ID %d: %v", domainProducts[i].ID, convErr)
			// Skip this product or return an error for the whole batch? For now, skipping.
			// To be robust, consider how to handle partial failures in a list.
			continue
		}
		protoProducts[i] = convertedProduct
	}
	// Filter out nil entries if any conversion failed and we continued
    actualProtoProducts := make([]*productpb.Product, 0, len(protoProducts))
    for _, p := range protoProducts {
        if p != nil {
            actualProtoProducts = append(actualProtoProducts, p)
        }
    }


	var nextPageToken string
	if offset+len(actualProtoProducts) < totalCount {
		nextPageToken = strconv.Itoa(offset + len(actualProtoProducts))
	}

	log.Printf("INFO: Returning %d products, total available: %d, next page token: '%s'", len(actualProtoProducts), totalCount, nextPageToken)
	return &productpb.ListProductsInternalResponse{
		Products: actualProtoProducts,
		PageInfo: &commonpb.PageInfoResponse{
			NextPageToken: nextPageToken,
			TotalSize:     int32(totalCount),
		},
	}, nil
}

func (s *GRPCHandler) UpdateStock(ctx context.Context, req *productpb.UpdateStockRequest) (*productpb.UpdateStockResponse, error) {
	log.Printf("INFO: Received gRPC UpdateStock request with %d items. OrderID: '%s'", len(req.GetItems()), req.GetOrderId())
	if len(req.GetItems()) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "No items provided for stock update")
	}

	// IMPORTANT: This iterative approach is NOT ATOMIC.
	// For production, a batch update method in the store layer that handles all items
	// within a single transaction is highly recommended for atomicity and performance.
	updatedProductsProto := make([]*productpb.Product, 0, len(req.GetItems()))
	var firstError error

	for _, item := range req.GetItems() {
		if item.GetProductId() <= 0 {
			log.Printf("WARN: Invalid Product ID %d in UpdateStock item", item.GetProductId())
			if firstError == nil { // Capture first error
				firstError = status.Errorf(codes.InvalidArgument, "Item has invalid Product ID: %d", item.GetProductId())
			}
			continue // Or fail entire batch
		}
		// quantityChange is int32, matches store method
		domainProduct, err := s.productStore.UpdateStock(ctx, item.GetProductId(), item.GetQuantityChange())
		if err != nil {
			log.Printf("ERROR: Failed to update stock for product ID %d: %v", item.GetProductId(), err)
			// Map specific errors like NotFound or InsufficientStock
			grpcErr := mapStoreErrorToGrpcStatus(err, "Product", item.GetProductId())
			if firstError == nil {
				firstError = grpcErr
			}
			// Decide on batch failure strategy: stop on first error, or collect all errors?
			// For now, we'll continue processing other items but return the first significant error.
			// The response will only contain successfully updated products.
			continue
		}
		protoProd, convErr := convertDomainProductToProto(domainProduct)
		if convErr != nil {
			log.Printf("ERROR: Failed to convert updated product ID %d to proto: %v", domainProduct.ID, convErr)
			if firstError == nil {
				firstError = status.Errorf(codes.Internal, "Failed to process data for product ID %d", domainProduct.ID)
			}
			continue
		}
		updatedProductsProto = append(updatedProductsProto, protoProd)
	}

	if firstError != nil && len(updatedProductsProto) < len(req.GetItems()) {
		// Partial success, but an error occurred. Return the error.
		// The client can inspect updated_products to see which ones succeeded.
		log.Printf("WARN: UpdateStock finished with partial success and an error: %v", firstError)
		// To be more granular, the response could include per-item statuses.
		// For now, if any error, we return it.
		return &productpb.UpdateStockResponse{UpdatedProducts: updatedProductsProto}, firstError
	}
	if firstError != nil { // All items failed
		return nil, firstError
	}


	log.Printf("INFO: Successfully updated stock for %d items.", len(updatedProductsProto))
	return &productpb.UpdateStockResponse{
		UpdatedProducts: updatedProductsProto,
	}, nil
}

func (s *GRPCHandler) CheckProductsAvailability(ctx context.Context, req *productpb.CheckProductsAvailabilityRequest) (*productpb.CheckProductsAvailabilityResponse, error) {
	log.Printf("INFO: Received gRPC CheckProductsAvailability request with %d items.", len(req.GetItems()))
	if len(req.GetItems()) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "No items provided for availability check")
	}

	productIDs := make([]int64, 0, len(req.GetItems()))
	requestedQuantities := make(map[int64]int32)
	for _, item := range req.GetItems() {
		if item.GetProductId() <= 0 {
			return nil, status.Errorf(codes.InvalidArgument, "Item contains invalid Product ID: %d", item.GetProductId())
		}
		productIDs = append(productIDs, item.GetProductId())
		requestedQuantities[item.GetProductId()] = item.GetRequiredQuantity()
		if item.GetRequiredQuantity() <=0 {
			return nil, status.Errorf(codes.InvalidArgument, "Item Product ID %d has invalid required quantity: %d", item.GetProductId(), item.GetRequiredQuantity())
		}
	}

	// Fetch all requested products in one go if possible (using ListProducts with ProductIDs filter)
	// We need active products only for availability check.
	isActiveTrue := true
	domainProducts, _, err := s.productStore.ListProducts(ctx, store.ListProductsParams{
		ProductIDs: productIDs,
		IsActive:   &isActiveTrue, // Typically, only check availability for active products
		Limit:      len(productIDs), // Ensure we try to fetch all
		Offset:     0,
	})
	if err != nil {
		log.Printf("ERROR: Failed to fetch products for availability check: %v", err)
		return nil, status.Errorf(codes.Internal, "Error retrieving product data for availability check")
	}

	// Create a map for quick lookup of fetched domain products
	domainProductMap := make(map[int64]domain.Product, len(domainProducts))
	for _, p := range domainProducts {
		domainProductMap[p.ID] = p
	}

	statuses := make([]*productpb.ProductAvailabilityStatus, 0, len(req.GetItems()))
	for _, item := range req.GetItems() {
		productID := item.GetProductId()
		requiredQty := item.GetRequiredQuantity()
		statusEntry := &productpb.ProductAvailabilityStatus{
			ProductId:    productID,
			IsAvailable:  false, // Default to not available
		}

		domainProd, found := domainProductMap[productID]
		if !found {
			reason := "Product not found."
			statusEntry.ReasonNotAvailable = &reason
			log.Printf("WARN: Product ID %d not found during availability check.", productID)
		} else {
			statusEntry.Name = domainProd.Name
			statusEntry.CurrentPrice = domainProd.Price // domain.Price is float64, proto is double
			statusEntry.AvailableQuantity = domainProd.StockQuantity // domain.StockQuantity is int32, proto is int32

			if !domainProd.IsActive {
				reason := "Product is not active."
				statusEntry.ReasonNotAvailable = &reason
				log.Printf("INFO: Product ID %d is not active during availability check.", productID)
			} else if domainProd.StockQuantity < requiredQty {
				reason := "Insufficient stock."
				statusEntry.ReasonNotAvailable = &reason
				log.Printf("INFO: Product ID %d has insufficient stock (%d available, %d required).", productID, domainProd.StockQuantity, requiredQty)
			} else {
				statusEntry.IsAvailable = true
				log.Printf("INFO: Product ID %d is available (stock: %d, required: %d).", productID, domainProd.StockQuantity, requiredQty)
			}
		}
		statuses = append(statuses, statusEntry)
	}

	log.Printf("INFO: Completed availability check for %d items.", len(statuses))
	return &productpb.CheckProductsAvailabilityResponse{
		Statuses: statuses,
	}, nil
}

// --- Helper Functions for Conversion ---

func convertDomainCategoryToProto(domainCat *domain.Category) *productpb.Category {
	if domainCat == nil {
		return nil
	}
	pbCat := &productpb.Category{
		Id:        domainCat.ID,
		Name:      domainCat.Name,
		CreatedAt: timestamppb.New(domainCat.CreatedAt),
		UpdatedAt: timestamppb.New(domainCat.UpdatedAt),
	}
	if domainCat.Description != nil {
		pbCat.Description = domainCat.Description
	}
	if domainCat.ParentCategoryID != nil {
		pbCat.ParentCategoryId = domainCat.ParentCategoryID
	}
	return pbCat
}

func convertDomainProductToProto(domainProd *domain.Product) (*productpb.Product, error) {
	if domainProd == nil {
		return nil, nil
	}

	pbProd := &productpb.Product{
		Id:             domainProd.ID,
		Name:           domainProd.Name,
		Sku:            domainProd.SKU,
		Price:          domainProd.Price, // float64 to double is fine
		StockQuantity:  domainProd.StockQuantity, // int32 to int32
		IsActive:       domainProd.IsActive,
		CreatedAt:      timestamppb.New(domainProd.CreatedAt),
		UpdatedAt:      timestamppb.New(domainProd.UpdatedAt),
	}

	if domainProd.Description != nil {
		pbProd.Description = domainProd.Description
	}
	if domainProd.CategoryID != nil {
		pbProd.CategoryId = domainProd.CategoryID
	}
	if domainProd.ImageURL != nil {
		pbProd.ImageUrl = domainProd.ImageURL
	}

	if domainProd.Attributes != nil && len(*domainProd.Attributes) > 0 {
		// Ensure it's not just "null" as a string from the DB if sql.NullString was used
        if string(*domainProd.Attributes) == "null" {
             // Treat as no attributes or handle as needed
        } else {
            var attrMap map[string]interface{}
            if err := json.Unmarshal(*domainProd.Attributes, &attrMap); err != nil {
                log.Printf("ERROR: Failed to unmarshal product attributes JSON for product ID %d: %v. Raw: %s", domainProd.ID, err, string(*domainProd.Attributes))
                return nil, status.Errorf(codes.Internal, "Error processing product attributes for ID %d", domainProd.ID)
            }
            // Convert map[string]interface{} to *structpb.Struct
            s, err := structpb.NewStruct(attrMap)
            if err != nil {
                log.Printf("ERROR: Failed to convert attributes map to structpb.Struct for product ID %d: %v", domainProd.ID, err)
                return nil, status.Errorf(codes.Internal, "Error processing product attributes structure for ID %d", domainProd.ID)
            }
            pbProd.Attributes = s
        }
	}
	return pbProd, nil
}