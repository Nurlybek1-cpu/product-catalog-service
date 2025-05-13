// File: product-catalog-service/cmd/main.go
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"product-catalog-service/internal/api"
	"product-catalog-service/internal/config" // Using the robust config package
	"product-catalog-service/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq" 
    
	
	productpb "product-catalog-service/proto/v1/product"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"github.com/joho/godotenv"
)

const (
	defaultAppName = "ProductCatalogService" // App name for logger
)

func main() {
	err := godotenv.Load() // Loads .env from the current directory by default
    if err != nil {
        // Log that .env file was not found or couldn't be loaded, but don't make it fatal.
        // The application can still proceed if environment variables are set in other ways.
        log.Println("INFO: .env file not found or error loading, relying on system environment variables.")
    }
	// Initialize structured logger
	if err := godotenv.Load(); err != nil {
		log.Println("INFO: No .env file found or failed to load, relying on system environment")
	}
	logger := log.New(os.Stdout, fmt.Sprintf("[%s] ", defaultAppName), log.LstdFlags|log.Lshortfile|log.Lmicroseconds)
	logger.Println("INFO: Starting service...")

	// --- Configuration Loading ---
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("FATAL: Error loading configuration: %v", err)
	}
	logger.Printf("INFO: Configuration loaded for APP_ENV: %s, LogLevel: %s", cfg.AppEnv, cfg.LogLevel)

	// --- Database Connection ---
	// dbStore now directly holds *store.PostgresStore
	db, err := sql.Open("postgres", cfg.Postgres.DSN())
	if err != nil {
		logger.Fatalf("FATAL: Failed to initialize database connection: %v", err)
	}
	defer func() {
		// This defer is a fallback if setupDB or other parts fail before graceful shutdown takes over.
		// Graceful shutdown will also try to close it.
		if err := db.Close(); err != nil {
			logger.Printf("WARN: Error closing database on deferred cleanup: %v", err)
		}
	}()

	if err := db.PingContext(context.Background()); err != nil { // Ping DB to ensure connection is live
		logger.Fatalf("FATAL: Failed to ping database: %v", err)
	}
	// Apply connection pool settings from config

	logger.Println("INFO: Database connection established and configured successfully.")
	dbStore := store.NewPostgresStore(db) // Pass the *sql.DB to the store constructor

	// --- Initialize API Handlers ---
	httpAPIHandler := api.NewHTTPHandler(dbStore, dbStore) // dbStore implements both interfaces
	grpcAPIHandler := api.NewGRPCHandler(dbStore, dbStore) // dbStore implements both interfaces

	// --- Setup & Start HTTP Server ---
	httpRouter := chi.NewRouter()
	setupBaseMiddleware(httpRouter, logger)     // Basic middleware
	registerHealthCheck(httpRouter, logger, db) // Health check for HTTP
	httpAPIHandler.RegisterRoutes(httpRouter)   // Register service-specific routes (e.g., /api/v1/products)

	httpServer := &http.Server{
		Addr:         ":" + cfg.HttpServer.Port,
		Handler:      httpRouter,
		ReadTimeout:  cfg.HttpServer.TimeoutRead,
		WriteTimeout: cfg.HttpServer.TimeoutWrite,
		IdleTimeout:  cfg.HttpServer.TimeoutIdle,
	}

	go func() {
		logger.Printf("INFO: HTTP server listening on port %s", cfg.HttpServer.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("FATAL: HTTP server ListenAndServe error: %v", err)
		}
		logger.Println("INFO: HTTP server has stopped.")
	}()

	// --- Setup & Start gRPC Server ---
	grpcServer := setupGRPCServer(logger, grpcAPIHandler)
	grpcListener, err := net.Listen("tcp", ":"+cfg.GrpcServer.Port)
	if err != nil {
		logger.Fatalf("FATAL: Failed to listen for gRPC on port %s: %v", cfg.GrpcServer.Port, err)
	}

	go func() {
		logger.Printf("INFO: gRPC server listening on port %s", cfg.GrpcServer.Port)
		if err := grpcServer.Serve(grpcListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logger.Fatalf("FATAL: gRPC server Serve error: %v", err)
		}
		logger.Println("INFO: gRPC server has stopped.")
	}()

	// --- Graceful Shutdown ---
	shutdownComplete := make(chan struct{})
	go waitForShutdown(logger, httpServer, grpcServer, dbStore, shutdownComplete)

	<-shutdownComplete // Block until graceful shutdown is complete
	logger.Println("INFO: Service shutdown sequence finished.")
}

func setupBaseMiddleware(router *chi.Mux, logger *log.Logger) {
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	// Using chi's logger which is quite good.
	// You can customize its output if needed or use your own logger middleware.
	router.Use(middleware.Logger) // Chi's request logger
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(60 * time.Second)) // Default timeout for requests
	logger.Println("INFO: Base HTTP middleware registered.")
}

func registerHealthCheck(router *chi.Mux, logger *log.Logger, db *sql.DB) {
	healthPath := "/api/v1/healthz" // Simplified health check path
	router.Get(healthPath, func(w http.ResponseWriter, r *http.Request) {
		// Check DB connection as part of health
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		dbStatus := "healthy"
		if err := db.PingContext(ctx); err != nil {
			dbStatus = "unhealthy"
			logger.Printf("WARN: Health check DB ping failed: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // Always 200, but payload indicates detailed status
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "healthy",
			"serviceName": defaultAppName,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
			"database":    dbStatus,
		})
	})
	logger.Printf("INFO: HTTP health check registered at %s", healthPath)
}

func setupGRPCServer(logger *log.Logger, grpcAPIHandler *api.GRPCHandler) *grpc.Server {
	// TODO: Add gRPC interceptors for logging, metrics, auth, validation, etc.
	// Example (you'd need to import these packages):
	// serverOptions := []grpc.ServerOption{
	// 	grpc.ChainUnaryInterceptor(
	// 		grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
	// 		grpc_zap.UnaryServerInterceptor(yourZapLogger), // Replace with your structured logger
	// 		grpc_recovery.UnaryServerInterceptor(),
	// 		// Add auth interceptor if needed
	// 	),
	// }
	// s := grpc.NewServer(serverOptions...)

	s := grpc.NewServer() // Using default options for now

	productpb.RegisterProductCatalogServiceServer(s, grpcAPIHandler)
	logger.Println("INFO: ProductCatalogService gRPC service registered.")

	// Register gRPC Health Checking Protocol service.
	grpc_health_v1.RegisterHealthServer(s, health.NewServer())
	logger.Println("INFO: gRPC health check service registered.")

	// Enable gRPC server reflection (useful for tools like grpcurl).
	reflection.Register(s)
	logger.Println("INFO: gRPC reflection service registered.")

	return s
}

func waitForShutdown(
	logger *log.Logger,
	httpServer *http.Server,
	grpcServer *grpc.Server,
	dbStore *store.PostgresStore, // Or a generic interface with Close() if preferred
	shutdownComplete chan struct{},
) {
	defer close(shutdownComplete) // Ensure channel is closed when function exits

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	receivedSignal := <-sigChan
	logger.Printf("INFO: Received signal: %s. Starting graceful shutdown...", receivedSignal)

	// Create a context with a timeout for the shutdown process.
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()

	// Shutdown gRPC server
	// grpcServer.GracefulStop() stops the server from accepting new connections and waits
	// for existing RPCs to complete, or until the context times out.
	logger.Println("INFO: Attempting to gracefully shut down gRPC server...")
	stoppedGrpc := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stoppedGrpc)
	}()

	// Shutdown HTTP server
	// httpServer.Shutdown() gracefully shuts down the server without interrupting active connections.
	logger.Println("INFO: Attempting to gracefully shut down HTTP server...")
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Printf("WARN: HTTP server graceful shutdown failed: %v", err)
	} else {
		logger.Println("INFO: HTTP server gracefully shut down.")
	}

	// Wait for gRPC to finish shutting down or timeout
	select {
	case <-stoppedGrpc:
		logger.Println("INFO: gRPC server gracefully shut down.")
	case <-shutdownCtx.Done(): // If context times out before gRPC stops
		logger.Printf("WARN: gRPC server graceful shutdown timed out: %v", shutdownCtx.Err())
		logger.Println("INFO: Forcing gRPC server stop...")
		grpcServer.Stop() // Force stop if graceful failed or timed out
		logger.Println("INFO: gRPC server forced stop.")
	}

	// Close database connection pool
	if dbStore != nil {
		if err := dbStore.Close(); err != nil { // Assumes dbStore has a Close() method
			logger.Printf("WARN: Error closing database connection: %v", err)
		}
		// The underlying *sql.DB is also closed by dbStore.Close() if implemented correctly
	}

	logger.Println("INFO: Graceful shutdown sequence completed.")
}
