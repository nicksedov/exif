package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"exif-service/internal/application"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP SDK server with EXIF-specific tools.
type Server struct {
	server      *mcp.Server
	exifService *application.ExifService
	gpsWriter   *application.GPSWriter
}

// NewServer creates and configures the MCP server with all EXIF tools.
func NewServer(exifSvc *application.ExifService, gpsWriter *application.GPSWriter) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "exif-service",
		Version: "1.0.0",
	}, nil)

	s := &Server{
		server:      srv,
		exifService: exifSvc,
		gpsWriter:   gpsWriter,
	}

	s.registerReadTools()
	s.registerWriteTools()

	return srv
}

// NewHTTPHandler creates an HTTP handler that serves the MCP protocol over Streamable HTTP.
func NewHTTPHandler(exifSvc *application.ExifService, gpsWriter *application.GPSWriter) http.Handler {
	srv := NewServer(exifSvc, gpsWriter)
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return srv
	}, nil)
}



// --- Read Tools ---

func (s *Server) registerReadTools() {
	// read_exif: Read all EXIF fields from image file
	s.server.AddTool(&mcp.Tool{
		Name:        "read_exif",
		Description: "Read all EXIF fields from image file (camera, lens, ISO, aperture, shutter, focal length, date, orientation, color space, software)",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Absolute path to the image file"},
			},
			"required": []string{"path"},
		},
	}, s.handleReadExif)

	// read_gps: Read GPS coordinates from image EXIF
	s.server.AddTool(&mcp.Tool{
		Name:        "read_gps",
		Description: "Read GPS coordinates from image EXIF (latitude, longitude, altitude if available)",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Absolute path to the image file"},
			},
			"required": []string{"path"},
		},
	}, s.handleReadGPS)

	// read_all_metadata: Read complete EXIF tag dump
	s.server.AddTool(&mcp.Tool{
		Name:        "read_all_metadata",
		Description: "Read complete EXIF tag dump (all tags, raw values)",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Absolute path to the image file"},
			},
			"required": []string{"path"},
		},
	}, s.handleReadAllMetadata)
}

func (s *Server) handleReadExif(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	meta, err := s.exifService.ExtractMetadata(args.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read EXIF: %w", err)
	}

	result := map[string]interface{}{
		"width":        meta.Width,
		"height":       meta.Height,
		"cameraModel":  meta.CameraModel,
		"lensModel":    meta.LensModel,
		"iso":          meta.ISO,
		"aperture":     meta.Aperture,
		"shutterSpeed": meta.ShutterSpeed,
		"focalLength":  meta.FocalLength,
		"orientation":  meta.Orientation,
		"colorSpace":   meta.ColorSpace,
		"software":     meta.Software,
	}
	if meta.DateTaken != nil {
		result["dateTaken"] = meta.DateTaken.Format("2006-01-02T15:04:05Z")
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}

func (s *Server) handleReadGPS(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	lat, lng, ok := s.exifService.ExtractGPS(args.Path)
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "No GPS data found in image EXIF"}},
		}, nil
	}

	result := map[string]interface{}{
		"latitude":  lat,
		"longitude": lng,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}

func (s *Server) handleReadAllMetadata(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	tags, err := s.exifService.ReadAllTags(args.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	data, _ := json.MarshalIndent(tags, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}

// --- Write Tools ---

func (s *Server) registerWriteTools() {
	// write_gps: Write GPS coordinates to image EXIF
	s.server.AddTool(&mcp.Tool{
		Name:        "write_gps",
		Description: "Write GPS coordinates to image EXIF (3-attempt strategy with backup)",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":      map[string]any{"type": "string", "description": "Absolute path to the image file"},
				"latitude":  map[string]any{"type": "number", "description": "GPS latitude (-90 to 90)"},
				"longitude": map[string]any{"type": "number", "description": "GPS longitude (-180 to 180)"},
				"backup_dir": map[string]any{"type": "string", "description": "Directory where a backup copy of the original file will be stored before modification"},
			},
			"required": []string{"path", "latitude", "longitude", "backup_dir"},
		},
	}, s.handleWriteGPS)

	// write_exif_field: Write arbitrary EXIF tag value
	s.server.AddTool(&mcp.Tool{
		Name:        "write_exif_field",
		Description: "Write arbitrary EXIF tag value (e.g., DateTimeOriginal, ImageDescription)",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":  map[string]any{"type": "string", "description": "Absolute path to the image file"},
				"tag":   map[string]any{"type": "string", "description": "EXIF tag name (e.g., DateTimeOriginal)"},
				"value": map[string]any{"type": "string", "description": "Value to write"},
			},
			"required": []string{"path", "tag", "value"},
		},
	}, s.handleWriteExifField)

	// strip_exif: Remove specified EXIF tags
	s.server.AddTool(&mcp.Tool{
		Name:        "strip_exif",
		Description: "Remove specified EXIF tags (or all if tags omitted)",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Absolute path to the image file"},
				"tags": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "List of EXIF tags to remove (omit to remove all)"},
			},
			"required": []string{"path"},
		},
	}, s.handleStripExif)

	// copy_exif: Copy EXIF data between files
	s.server.AddTool(&mcp.Tool{
		Name:        "copy_exif",
		Description: "Copy EXIF data from source to target file",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"source_path": map[string]any{"type": "string", "description": "Source file path"},
				"target_path": map[string]any{"type": "string", "description": "Target file path"},
				"tags":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Specific tags to copy (omit to copy all)"},
			},
			"required": []string{"source_path", "target_path"},
		},
	}, s.handleCopyExif)

	// compare_exif: Compare EXIF between two images
	s.server.AddTool(&mcp.Tool{
		Name:        "compare_exif",
		Description: "Compare EXIF metadata between two images, return differences",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path1": map[string]any{"type": "string", "description": "Path to first image"},
				"path2": map[string]any{"type": "string", "description": "Path to second image"},
			},
			"required": []string{"path1", "path2"},
		},
	}, s.handleCompareExif)

	// validate_exif: Validate EXIF integrity
	s.server.AddTool(&mcp.Tool{
		Name:        "validate_exif",
		Description: "Validate EXIF integrity (check for corruption, InteropIFD issues)",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Absolute path to the image file"},
			},
			"required": []string{"path"},
		},
	}, s.handleValidateExif)
}

func (s *Server) handleWriteGPS(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Path      string  `json:"path"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		BackupDir string  `json:"backup_dir"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if err := s.gpsWriter.WriteGPS(args.Path, args.Latitude, args.Longitude, nil, args.BackupDir); err != nil {
		return nil, fmt.Errorf("GPS write failed: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("GPS coordinates written to %s: lat=%.8f, lng=%.8f", args.Path, args.Latitude, args.Longitude)}},
	}, nil
}

func (s *Server) handleWriteExifField(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Path  string `json:"path"`
		Tag   string `json:"tag"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if err := application.WriteExifField(args.Path, args.Tag, args.Value); err != nil {
		return nil, fmt.Errorf("EXIF field write failed: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("EXIF field %s=%s written to %s", args.Tag, args.Value, args.Path)}},
	}, nil
}

func (s *Server) handleStripExif(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Path string   `json:"path"`
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if err := application.StripExif(args.Path, args.Tags); err != nil {
		return nil, fmt.Errorf("EXIF strip failed: %w", err)
	}

	msg := fmt.Sprintf("EXIF stripped from %s", args.Path)
	if len(args.Tags) > 0 {
		msg += fmt.Sprintf(" (tags: %v)", args.Tags)
	} else {
		msg += " (all tags)"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}, nil
}

func (s *Server) handleCopyExif(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		SourcePath string   `json:"source_path"`
		TargetPath string   `json:"target_path"`
		Tags       []string `json:"tags"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if err := application.CopyExif(args.SourcePath, args.TargetPath, args.Tags); err != nil {
		return nil, fmt.Errorf("EXIF copy failed: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("EXIF copied from %s to %s", args.SourcePath, args.TargetPath)}},
	}, nil
}

func (s *Server) handleCompareExif(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Path1 string `json:"path1"`
		Path2 string `json:"path2"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	meta1, err := s.exifService.ExtractMetadata(args.Path1)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata from %s: %w", args.Path1, err)
	}
	meta2, err := s.exifService.ExtractMetadata(args.Path2)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata from %s: %w", args.Path2, err)
	}

	diff := map[string]interface{}{
		"cameraModel":  compareField(meta1.CameraModel, meta2.CameraModel),
		"lensModel":    compareField(meta1.LensModel, meta2.LensModel),
		"iso":          compareField(meta1.ISO, meta2.ISO),
		"aperture":     compareField(meta1.Aperture, meta2.Aperture),
		"shutterSpeed": compareField(meta1.ShutterSpeed, meta2.ShutterSpeed),
		"focalLength":  compareField(meta1.FocalLength, meta2.FocalLength),
		"orientation":  compareField(meta1.Orientation, meta2.Orientation),
		"colorSpace":   compareField(meta1.ColorSpace, meta2.ColorSpace),
		"software":     compareField(meta1.Software, meta2.Software),
	}

	data, _ := json.MarshalIndent(diff, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}

func (s *Server) handleValidateExif(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Try to read all tags - failures indicate corruption
	_, err := s.exifService.ReadAllTags(args.Path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("EXIF validation FAILED for %s: %s", args.Path, err.Error())}},
		}, nil
	}

	// Try extracting structured metadata
	meta, err := s.exifService.ExtractMetadata(args.Path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("EXIF partially valid for %s: metadata extraction failed: %s", args.Path, err.Error())}},
		}, nil
	}

	status := "VALID"
	if !application.HasExifData(meta) {
		status = "NO_EXIF_DATA"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("EXIF %s for %s (width=%d, height=%d)", status, args.Path, meta.Width, meta.Height)}},
	}, nil
}

// --- helpers ---

func compareField(v1, v2 interface{}) map[string]interface{} {
	same := fmt.Sprintf("%v", v1) == fmt.Sprintf("%v", v2)
	return map[string]interface{}{
		"image1": v1,
		"image2": v2,
		"same":   same,
	}
}
