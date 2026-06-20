package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"exif-service/internal/application"
	"exif-service/internal/infrastructure/config"
	"exif-service/internal/infrastructure/database"
	"exif-service/internal/interfaces/handler"
	exifmcp "exif-service/mcp"
)

func main() {
	cfg := config.Load()

	fmt.Println("EXIF Microservice")
	fmt.Println("==================")

	// Initialize database
	fmt.Println("Connecting to PostgreSQL database...")
	db, err := database.Init(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	fmt.Println("Database connected successfully!")

	// Initialize exiftool
	fmt.Println("Initializing exiftool...")
	exifSvc, err := application.NewExifService()
	if err != nil {
		log.Fatalf("Failed to initialize exiftool: %v", err)
	}
	fmt.Println("exiftool initialized!")

	// Initialize GPS writer
	gpsWriter := application.NewGPSWriter()

	// Initialize MCP server
	mcpHTTPHandler := exifmcp.NewHTTPHandler(exifSvc, gpsWriter)
	fmt.Println("MCP server initialized with EXIF tools")

	// Set up Gin router with all routes
	router := handler.SetupRouter(db, exifSvc, gpsWriter)

	// Mount MCP endpoint
	router.Any("/exif/mcp", gin.WrapH(mcpHTTPHandler))

	fmt.Printf("\nStarting EXIF service on http://%s:%s\n", cfg.ServerHost, cfg.ServerPort)
	fmt.Println("REST API: /exif/*")
	fmt.Println("MCP endpoint: /exif/mcp")
	fmt.Println("Press Ctrl+C to stop")

	addr := fmt.Sprintf("%s:%s", cfg.ServerHost, cfg.ServerPort)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal (SIGINT / SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	fmt.Printf("\nReceived signal %s, shutting down gracefully...\n", sig)

	// Graceful shutdown with a 10-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	fmt.Println("Server stopped.")
}
