# Stage 1: Build the Go application
# Use a Go version consistent with your go.mod (e.g., 1.22 or newer stable Alpine version)
FROM golang:1.22-alpine AS builder
# FROM golang:1.22.3-alpine AS builder # Or a specific patch version

# Install git for private modules or specific versions if needed by go mod download
# and build-base for CGO if it were ever enabled (though we disable it).
# For a pure Go project without CGO, these might not be strictly necessary
# but are good to have if any dependency pulls them in.
RUN apk --no-cache add git build-base

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files to download dependencies first.
# This leverages Docker's layer caching.
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy the rest of the application's source code.
# Consider using a .dockerignore file to exclude unnecessary files/folders
# (e.g., .git, .vscode, local .env files, temporary build artifacts).
COPY . .

# Build the Go application.
# -o /app/server: Output the compiled binary to /app/server
# CGO_ENABLED=0: Disable Cgo to build a statically linked binary (important for alpine)
# GOOS=linux: Specify the target operating system as Linux
# -ldflags="-w -s": Reduce binary size by stripping debug information
# -v: Verbose output, can be removed for cleaner logs once build is stable
RUN CGO_ENABLED=0 GOOS=linux go build -v -ldflags="-w -s" -o /app/server ./cmd/main.go

# Stage 2: Create a lightweight final image
# Use a specific version of Alpine for reproducibility, or latest if preferred.
FROM alpine:3.19
# FROM alpine:latest

# Install ca-certificates for HTTPS/TLS communication (e.g., to external APIs, or if gRPC uses TLS)
# and tzdata for timezone information if your application needs it.
RUN apk --no-cache add ca-certificates tzdata

# Create a non-root user and group for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the built binary from the 'builder' stage.
COPY --from=builder /app/server /app/server

# Switch to the non-root user
USER appuser

# Expose the port that your Go application will listen on for HTTP requests.
# This should match the HTTP_SERVER_PORT from your config (default 8081).
EXPOSE 8081

# Expose the port for gRPC.
# This should match the GRPC_SERVER_PORT from your config (default 9090).
EXPOSE 9090

# Command to run the executable when the container starts.
ENTRYPOINT ["/app/server"]
