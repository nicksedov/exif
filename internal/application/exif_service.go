package application

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"exif-service/internal/domain"

	"github.com/barasher/go-exiftool"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// ExifService provides EXIF metadata extraction and enrichment operations.
type ExifService struct {
	et *exiftool.Exiftool
}

// NewExifService creates a new ExifService. It checks for exiftool binary availability.
func NewExifService() (*ExifService, error) {
	if _, err := exec.LookPath("exiftool"); err != nil {
		return nil, fmt.Errorf("exiftool binary not found in PATH: %w", err)
	}

	et, err := exiftool.NewExiftool()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize exiftool: %w", err)
	}

	log.Printf("EXIF: go-exiftool initialized successfully")
	return &ExifService{et: et}, nil
}

// IsAvailable returns true if exiftool is initialized.
func (s *ExifService) IsAvailable() bool {
	return s != nil && s.et != nil
}

// ExtractMetadata reads EXIF metadata and image dimensions from a file.
func (s *ExifService) ExtractMetadata(filePath string) (*domain.ImageMetadata, error) {
	meta := &domain.ImageMetadata{}

	if w, h, err := getImageDimensions(filePath); err == nil {
		meta.Width = w
		meta.Height = h
	}

	if s.IsAvailable() {
		s.extractExifFields(filePath, meta)
	} else {
		log.Printf("EXIF: exiftool not initialized, skipping EXIF extraction for %s", filepath.Base(filePath))
	}

	return meta, nil
}

// ExtractGPS reads GPS coordinates from an image file's EXIF metadata.
func (s *ExifService) ExtractGPS(filePath string) (float64, float64, bool) {
	if !s.IsAvailable() {
		return 0, 0, false
	}

	fileInfos := s.et.ExtractMetadata(filePath)
	if len(fileInfos) == 0 || fileInfos[0].Err != nil {
		return 0, 0, false
	}

	fi := fileInfos[0]
	baseName := filepath.Base(filePath)

	// Method 1: Try direct GPSLatitude/GPSLongitude as float
	if lat, err := fi.GetFloat("GPSLatitude"); err == nil {
		if lng, err := fi.GetFloat("GPSLongitude"); err == nil {
			log.Printf("EXIF %s: GPS via float: lat=%.8f, lng=%.8f", baseName, lat, lng)
			return lat, lng, true
		}
	}

	// Method 2: Try GPSLatitude/GPSLongitude as string and parse
	if latStr, err := fi.GetString("GPSLatitude"); err == nil {
		if lngStr, err := fi.GetString("GPSLongitude"); err == nil {
			lat, latOk := parseGPSString(latStr)
			lng, lngOk := parseGPSString(lngStr)
			if latOk && lngOk {
				if ref, err := fi.GetString("GPSLatitudeRef"); err == nil && (ref == "S" || ref == "s") {
					lat = -lat
				}
				if ref, err := fi.GetString("GPSLongitudeRef"); err == nil && (ref == "W" || ref == "w") {
					lng = -lng
				}
				log.Printf("EXIF %s: GPS via string parse: lat=%.8f, lng=%.8f", baseName, lat, lng)
				return lat, lng, true
			}
		}
	}

	// Method 3: Try GPSPosition if available
	if gpsPos, err := fi.GetString("GPSPosition"); err == nil {
		if lat, lng, ok := parseGPSPosition(gpsPos); ok {
			log.Printf("EXIF %s: GPS via GPSPosition: lat=%.8f, lng=%.8f", baseName, lat, lng)
			return lat, lng, true
		}
	}

	return 0, 0, false
}

// HasExifData returns true if any meaningful EXIF field is populated.
func HasExifData(meta *domain.ImageMetadata) bool {
	return meta.CameraModel != "" || meta.LensModel != "" || meta.ISO != 0 ||
		meta.Aperture != "" || meta.ShutterSpeed != "" || meta.FocalLength != "" ||
		meta.DateTaken != nil || meta.Software != "" || meta.GeolocationRef != nil
}

// EnrichMissingMetadata reads EXIF from the file and fills any empty fields.
// Returns a map of field→value for the enriched fields, or nil if nothing was enriched.
func (s *ExifService) EnrichMissingMetadata(filePath string, meta *domain.ImageMetadata) map[string]interface{} {
	if meta.CameraModel != "" && meta.LensModel != "" && meta.ISO != 0 &&
		meta.Aperture != "" && meta.ShutterSpeed != "" && meta.FocalLength != "" &&
		meta.DateTaken != nil && meta.Orientation != 0 && meta.ColorSpace != "" && meta.Software != "" {
		return nil
	}

	if !s.IsAvailable() {
		return nil
	}

	fileInfos := s.et.ExtractMetadata(filePath)
	if len(fileInfos) == 0 || fileInfos[0].Err != nil {
		return nil
	}

	fi := fileInfos[0]
	baseName := filepath.Base(filePath)
	enriched := make(map[string]interface{})

	if meta.CameraModel == "" {
		if model, err := fi.GetString("Model"); err == nil && model != "" {
			meta.CameraModel = cleanString(model)
			enriched["camera_model"] = meta.CameraModel
			log.Printf("EXIF enrich %s: CameraModel=%s", baseName, meta.CameraModel)
		}
	}
	if meta.LensModel == "" {
		if lens, err := fi.GetString("LensModel"); err == nil && lens != "" {
			meta.LensModel = cleanString(lens)
			enriched["lens_model"] = meta.LensModel
		}
	}
	if meta.ISO == 0 {
		if iso, err := fi.GetInt("ISO"); err == nil {
			meta.ISO = int(iso)
			enriched["iso"] = meta.ISO
		}
	}
	if meta.Aperture == "" {
		if aperture, err := fi.GetFloat("FNumber"); err == nil {
			meta.Aperture = fmt.Sprintf("f/%.1f", aperture)
			enriched["aperture"] = meta.Aperture
		}
	}
	if meta.ShutterSpeed == "" {
		if exposureTime, err := fi.GetFloat("ExposureTime"); err == nil {
			meta.ShutterSpeed = formatExposureTimeFloat(exposureTime)
			enriched["shutter_speed"] = meta.ShutterSpeed
		}
	}
	if meta.FocalLength == "" {
		if focalLength, err := fi.GetFloat("FocalLength"); err == nil {
			if focalLength == math.Trunc(focalLength) {
				meta.FocalLength = fmt.Sprintf("%.0fmm", focalLength)
			} else {
				meta.FocalLength = fmt.Sprintf("%.1fmm", focalLength)
			}
			enriched["focal_length"] = meta.FocalLength
		}
	}
	if meta.DateTaken == nil {
		extractDateTaken(fi, meta, baseName)
		if meta.DateTaken != nil {
			enriched["date_taken"] = meta.DateTaken
		}
	}
	if meta.Orientation == 0 {
		if orientation, err := fi.GetString("Orientation"); err == nil && orientation != "" {
			meta.Orientation = parseOrientation(orientation)
			enriched["orientation"] = meta.Orientation
		}
	}
	if meta.ColorSpace == "" {
		if colorSpace, err := fi.GetString("ColorSpace"); err == nil && colorSpace != "" {
			meta.ColorSpace = parseColorSpace(colorSpace)
			enriched["color_space"] = meta.ColorSpace
		}
	}
	if meta.Software == "" {
		if software, err := fi.GetString("Software"); err == nil && software != "" {
			meta.Software = cleanString(software)
			enriched["software"] = meta.Software
		}
	}

	if len(enriched) == 0 {
		return nil
	}
	return enriched
}

// ReadAllTags returns a complete EXIF tag dump for a file.
func (s *ExifService) ReadAllTags(filePath string) (map[string]string, error) {
	if !s.IsAvailable() {
		return nil, fmt.Errorf("exiftool not available")
	}

	fileInfos := s.et.ExtractMetadata(filePath)
	if len(fileInfos) == 0 {
		return nil, fmt.Errorf("no metadata found for %s", filePath)
	}
	fi := fileInfos[0]
	if fi.Err != nil {
		return nil, fi.Err
	}

	result := make(map[string]string)
	for k, v := range fi.Fields {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result, nil
}

// --- internal helpers ---

func getImageDimensions(filePath string) (int, int, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

func (s *ExifService) extractExifFields(filePath string, meta *domain.ImageMetadata) {
	fileInfos := s.et.ExtractMetadata(filePath)
	if len(fileInfos) == 0 {
		return
	}

	fi := fileInfos[0]
	if fi.Err != nil {
		log.Printf("EXIF: Error extracting metadata from %s: %v", filepath.Base(filePath), fi.Err)
		return
	}

	baseName := filepath.Base(filePath)

	if model, err := fi.GetString("Model"); err == nil && model != "" {
		meta.CameraModel = cleanString(model)
		log.Printf("EXIF %s: CameraModel=%s", baseName, meta.CameraModel)
	}
	if lens, err := fi.GetString("LensModel"); err == nil && lens != "" {
		meta.LensModel = cleanString(lens)
	}
	if iso, err := fi.GetInt("ISO"); err == nil {
		meta.ISO = int(iso)
	}
	if aperture, err := fi.GetFloat("FNumber"); err == nil {
		meta.Aperture = fmt.Sprintf("f/%.1f", aperture)
	}
	if exposureTime, err := fi.GetFloat("ExposureTime"); err == nil {
		meta.ShutterSpeed = formatExposureTimeFloat(exposureTime)
	}
	if focalLength, err := fi.GetFloat("FocalLength"); err == nil {
		if focalLength == math.Trunc(focalLength) {
			meta.FocalLength = fmt.Sprintf("%.0fmm", focalLength)
		} else {
			meta.FocalLength = fmt.Sprintf("%.1fmm", focalLength)
		}
	}
	extractDateTaken(fi, meta, baseName)
	if orientation, err := fi.GetString("Orientation"); err == nil && orientation != "" {
		meta.Orientation = parseOrientation(orientation)
	}
	if colorSpace, err := fi.GetString("ColorSpace"); err == nil && colorSpace != "" {
		meta.ColorSpace = parseColorSpace(colorSpace)
	}
	if software, err := fi.GetString("Software"); err == nil && software != "" {
		meta.Software = cleanString(software)
	}
}

func extractDateTaken(fi exiftool.FileMetadata, meta *domain.ImageMetadata, baseName string) {
	dateFields := []string{"DateTimeOriginal", "CreateDate", "ModifyDate", "DateTime"}
	for _, field := range dateFields {
		if dateStr, err := fi.GetString(field); err == nil && dateStr != "" {
			if t, err := parseExifDate(dateStr); err == nil {
				meta.DateTaken = &t
				log.Printf("EXIF %s: DateTaken=%s (from %s)", baseName, t.Format("2006-01-02 15:04:05"), field)
				return
			}
		}
	}
}

func parseGPSString(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	s = strings.TrimRight(s, "NSEWnesw ")
	if val, err := strconv.ParseFloat(s, 64); err == nil {
		return val, true
	}
	s = strings.ReplaceAll(s, "deg", "")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.TrimSpace(s)

	parts := strings.Fields(s)
	if len(parts) >= 2 {
		deg, err1 := strconv.ParseFloat(parts[0], 64)
		min, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil {
			sec := 0.0
			if len(parts) >= 3 {
				sec, _ = strconv.ParseFloat(parts[2], 64)
			}
			return deg + min/60.0 + sec/3600.0, true
		}
	}
	return 0, false
}

func parseGPSPosition(s string) (float64, float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}
	parts := strings.Split(s, ",")
	if len(parts) == 2 {
		lat, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		lng, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 == nil && err2 == nil {
			return lat, lng, true
		}
	}
	parts = strings.Fields(s)
	if len(parts) == 2 {
		lat, err1 := strconv.ParseFloat(parts[0], 64)
		lng, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil {
			return lat, lng, true
		}
	}
	return 0, 0, false
}

func parseOrientation(s string) int {
	s = strings.TrimSpace(s)
	if val, err := strconv.Atoi(s); err == nil {
		return val
	}
	lower := strings.ToLower(s)
	if strings.Contains(lower, "rotate") {
		if strings.Contains(lower, "90") {
			if strings.Contains(lower, "cw") || strings.Contains(lower, "normal") {
				return 6
			}
			return 8
		}
		if strings.Contains(lower, "180") {
			return 3
		}
	}
	if strings.Contains(lower, "mirror") || strings.Contains(lower, "flip") {
		if strings.Contains(lower, "horizontal") {
			return 2
		}
		if strings.Contains(lower, "vertical") {
			return 4
		}
	}
	if strings.Contains(lower, "normal") || strings.Contains(lower, "horizontal") {
		return 1
	}
	return 0
}

func parseColorSpace(s string) string {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)
	switch {
	case lower == "srgb", strings.Contains(lower, "srgb"):
		return "sRGB"
	case lower == "adobe rgb", strings.Contains(lower, "adobe"):
		return "Adobe RGB"
	case strings.Contains(lower, "uncalibrat"):
		return "Uncalibrated"
	default:
		return s
	}
}

func parseExifDate(dateStr string) (time.Time, error) {
	dateStr = strings.TrimSpace(dateStr)
	if len(dateStr) < 19 {
		return time.Time{}, fmt.Errorf("invalid date format: %s", dateStr)
	}
	dateStr = dateStr[:4] + "-" + dateStr[5:7] + "-" + dateStr[8:]
	return time.Parse("2006-01-02 15:04:05", dateStr)
}

func cleanString(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "\x00")
	return s
}

func formatExposureTimeFloat(val float64) string {
	if val <= 0 {
		return "0s"
	}
	if val >= 1 {
		if val == math.Trunc(val) {
			return fmt.Sprintf("%.0fs", val)
		}
		return fmt.Sprintf("%.1fs", val)
	}
	denom := int(1.0 / val)
	return fmt.Sprintf("1/%ds", denom)
}
