module product-catalog-service

go 1.22 // Or your team's agreed-upon Go version (e.g., 1.23 if using Go 1.23 features)

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/go-chi/chi/v5 v5.0.12
	github.com/go-playground/validator/v10 v10.20.0
	github.com/kelseyhightower/envconfig v1.3.0
	github.com/lib/pq v1.10.9
	github.com/stretchr/testify v1.9.0
	google.golang.org/grpc v1.64.0
	google.golang.org/protobuf v1.34.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.3 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/joho/godotenv v1.5.1
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/net v0.22.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Indirect dependencies will be populated/updated by 'go mod tidy'.
// If you were using well-known types like google.rpc.Status directly from genproto,
// you might have an explicit require for google.golang.org/genproto, but for
// timestamppb, structpb, etc., google.golang.org/protobuf/types/known/... is usually sufficient.
// For example:
// require google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a14d17d8c6 // (Only if directly used and not transitively required)
