package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"exif-service/internal/application"
	"exif-service/internal/infrastructure/config"
	"exif-service/internal/infrastructure/database"
	"exif-service/internal/interfaces/handler"
	exifmcp "exif-service/mcp"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found")
	}
}

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
	gpsWriter := application.NewGPSWriter(cfg.TrashDir)

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

	if err := router.Run(fmt.Sprintf("%s:%s", cfg.ServerHost, cfg.ServerPort)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// unused but needed for type resolution
var _ http.Handler
