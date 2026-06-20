package handler

import (
	"math"
	"net/http"
	"path/filepath"
	"slices"
	"time"

	"exif-service/internal/application"
	"exif-service/internal/interfaces/dto"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler holds all dependencies for the EXIF service REST API.
type Handler struct {
	db          *gorm.DB
	exifService *application.ExifService
	gpsWriter   *application.GPSWriter
	startTime   time.Time
}

// NewHandler creates a new handler instance.
func NewHandler(db *gorm.DB, exifSvc *application.ExifService, gpsWriter *application.GPSWriter) *Handler {
	return &Handler{
		db:          db,
		exifService: exifSvc,
		gpsWriter:   gpsWriter,
		startTime:   time.Now(),
	}
}

// HandleHealth returns the service health status.
func (h *Handler) HandleHealth(c *gin.Context) {
	dbConnected := true
	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.Ping() != nil {
		dbConnected = false
	}

	c.JSON(http.StatusOK, dto.HealthResponse{
		Status:            "healthy",
		Version:           "1.0.0",
		ExiftoolAvailable: h.exifService.IsAvailable(),
		DatabaseConnected: dbConnected,
		Uptime:            time.Since(h.startTime).Round(time.Second).String(),
	})
}

// HandleGetMetadata reads EXIF metadata from a file.
func (h *Handler) HandleGetMetadata(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "path query parameter is required"})
		return
	}

	meta, err := h.exifService.ExtractMetadata(filepath.FromSlash(path))
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	resp := dto.MetadataResponse{
		Width:        meta.Width,
		Height:       meta.Height,
		CameraModel:  meta.CameraModel,
		LensModel:    meta.LensModel,
		ISO:          meta.ISO,
		Aperture:     meta.Aperture,
		ShutterSpeed: meta.ShutterSpeed,
		FocalLength:  meta.FocalLength,
		Orientation:  meta.Orientation,
		ColorSpace:   meta.ColorSpace,
		Software:     meta.Software,
	}

	if meta.DateTaken != nil {
		resp.DateTaken = meta.DateTaken.Format(time.RFC3339)
	}

	// Try to extract GPS
	if lat, lng, ok := h.exifService.ExtractGPS(filepath.FromSlash(path)); ok {
		resp.GPSLatitude = &lat
		resp.GPSLongitude = &lng
	}

	c.JSON(http.StatusOK, resp)
}

// HandleUpdateGPS writes GPS coordinates to a single image file.
func (h *Handler) HandleUpdateGPS(c *gin.Context) {
	var req dto.GPSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body"})
		return
	}

	osPath := filepath.FromSlash(req.Path)
	if err := h.gpsWriter.WriteGPS(osPath, req.Latitude, req.Longitude, nil); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.GPSResponse{
		Success:   true,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
	})
}

// HandleBatchUpdateGPS writes GPS coordinates to multiple images.
func (h *Handler) HandleBatchUpdateGPS(c *gin.Context) {
	var req dto.GPSBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body"})
		return
	}

	var successCount, failedCount int
	var failedFiles []string

	for _, item := range req.Items {
		osPath := filepath.FromSlash(item.Path)
		if err := h.gpsWriter.WriteGPS(osPath, item.Latitude, item.Longitude, nil); err != nil {
			failedCount++
			failedFiles = append(failedFiles, item.Path)
			continue
		}
		successCount++
	}

	c.JSON(http.StatusOK, dto.GPSBatchResponse{
		Success:     successCount,
		Failed:      failedCount,
		FailedFiles: failedFiles,
	})
}

// HandleGetLocationCandidates returns location suggestions from same-day photos.
// Requires a date parameter (YYYY-MM-DD). The external service is responsible for
// resolving file paths to dates — this endpoint only queries image_metadata + geolocation_caches.
func (h *Handler) HandleGetLocationCandidates(c *gin.Context) {
	dateParam := c.Query("date")
	if dateParam == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "date query parameter is required (YYYY-MM-DD)"})
		return
	}

	if _, err := time.Parse("2006-01-02", dateParam); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid date format, expected YYYY-MM-DD"})
		return
	}

	targetDate, _ := time.Parse("2006-01-02", dateParam)
	nextDay := targetDate.AddDate(0, 0, 1)

	type gpsRow struct {
		GPSLatitude  float64
		GPSLongitude float64
		NameLocal    string
		NameEng      string
	}

	var rows []gpsRow
	h.db.Table("image_metadata").
		Select("geolocation_caches.gps_latitude, geolocation_caches.gps_longitude, geolocation_caches.name_local, geolocation_caches.name_eng").
		Joins("JOIN geolocation_caches ON geolocation_caches.id = image_metadata.geolocation_ref").
		Where("image_metadata.date_taken >= ? AND image_metadata.date_taken < ?", targetDate, nextDay).
		Limit(200).
		Find(&rows)

	if len(rows) == 0 {
		c.JSON(http.StatusOK, dto.LocationCandidatesResponse{Candidates: []dto.LocationCandidate{}})
		return
	}

	type locationKey struct {
		Lat float64
		Lng float64
	}
	type locationGroup struct {
		LatSum     float64
		LngSum     float64
		NameLocal  string
		NameEng    string
		PhotoCount int
	}

	groupMap := make(map[locationKey]*locationGroup)
	var order []locationKey

	for _, r := range rows {
		roundedLat := math.Round(r.GPSLatitude*20) / 20
		roundedLng := math.Round(r.GPSLongitude*20) / 20
		key := locationKey{Lat: roundedLat, Lng: roundedLng}

		if g, ok := groupMap[key]; ok {
			g.LatSum += r.GPSLatitude
			g.LngSum += r.GPSLongitude
			g.PhotoCount++
		} else {
			groupMap[key] = &locationGroup{
				LatSum:     r.GPSLatitude,
				LngSum:     r.GPSLongitude,
				NameLocal:  r.NameLocal,
				NameEng:    r.NameEng,
				PhotoCount: 1,
			}
			order = append(order, key)
		}
	}

	candidates := make([]dto.LocationCandidate, 0, len(order))
	for i, key := range order {
		if i >= 20 {
			break
		}
		g := groupMap[key]
		candidates = append(candidates, dto.LocationCandidate{
			Lat:        g.LatSum / float64(g.PhotoCount),
			Lng:        g.LngSum / float64(g.PhotoCount),
			NameLocal:  g.NameLocal,
			NameEng:    g.NameEng,
			PhotoCount: g.PhotoCount,
		})
	}

	slices.SortFunc(candidates, func(a, b dto.LocationCandidate) int {
		return b.PhotoCount - a.PhotoCount
	})

	c.JSON(http.StatusOK, dto.LocationCandidatesResponse{Candidates: candidates})
}

// SetupRouter creates the Gin router with all EXIF service routes.
func SetupRouter(db *gorm.DB, exifSvc *application.ExifService, gpsWriter *application.GPSWriter) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, PUT, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	h := NewHandler(db, exifSvc, gpsWriter)

	exif := r.Group("/exif")
	{
		exif.GET("/health", h.HandleHealth)
		exif.GET("/metadata", h.HandleGetMetadata)
		exif.PUT("/gps", h.HandleUpdateGPS)
		exif.PUT("/gps/batch", h.HandleBatchUpdateGPS)
		exif.GET("/location-candidates", h.HandleGetLocationCandidates)
	}

	return r
}
