package domain

import (
	"encoding/json" // For json.RawMessage if you choose to use it for Attributes
	"time"
)

// Category represents a product category in the system.
// The json tags correspond to the fields expected in API responses/requests.
type Category struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	Description      *string    `json:"description,omitempty"`      // Pointer for nullable fields, omitempty to exclude if nil
	ParentCategoryID *int64     `json:"parent_category_id,omitempty"` // Pointer for nullable fields
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// Product represents a product in the catalog.
// The json tags correspond to the fields expected in API responses/requests.
type Product struct {
	ID             int64            `json:"id"`
	Name           string           `json:"name"`
	Description    *string          `json:"description,omitempty"`    // Pointer for nullable fields
	SKU            string           `json:"sku"`
	Price          float64          `json:"price"`                    // For currency, consider using a dedicated decimal type library in production for precision
	StockQuantity  int32            `json:"stock_quantity"`
	CategoryID     *int64           `json:"category_id,omitempty"`    // Pointer for nullable fields
	ImageURL       *string          `json:"image_url,omitempty"`      // Pointer for nullable fields
	IsActive       bool             `json:"is_active"`
	Attributes     *json.RawMessage `json:"attributes,omitempty"`     // For JSONB. Use json.RawMessage to defer parsing.
	                                                                // Alternatively, use *map[string]interface{}
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// Note on Product.Attributes:
// Using *json.RawMessage for the Attributes field is a flexible way to handle JSONB data.
// - When reading from the DB: The raw JSON bytes can be scanned into this field.
// - When sending in API response: It will be marshalled as is.
// - When receiving in API request: It will be unmarshalled as raw JSON bytes.
// Your service logic can then unmarshal it into a specific struct if needed:
//   var specificAttrs SomeSpecificAttributeStruct
//   if product.Attributes != nil && len(*product.Attributes) > 0 {
//       if err := json.Unmarshal(*product.Attributes, &specificAttrs); err != nil {
//           // handle error
//       }
//   }
// Or, if you prefer to work with it as a map directly in your domain:
// Attributes     *map[string]interface{} `json:"attributes,omitempty"`
// The pq driver for PostgreSQL can often map map[string]interface{} to JSONB directly.
// Choose the approach that best fits your needs for handling these flexible attributes.
// json.RawMessage is good if you often pass it through without needing to inspect its contents deeply in Go.
