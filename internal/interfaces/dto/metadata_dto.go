package dto

// HealthResponse is the JSON response for GET /exif/health
type HealthResponse struct {
	Status            string `json:"status"`
	Version           string `json:"version"`
	ExiftoolAvailable bool   `json:"exiftoolAvailable"`
	DatabaseConnected bool   `json:"databaseConnected"`
	Uptime            string `json:"uptime"`
}

// MetadataResponse is the JSON response for GET /exif/metadata
type MetadataResponse struct {
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	CameraModel  string   `json:"cameraModel"`
	LensModel    string   `json:"lensModel"`
	ISO          int      `json:"iso"`
	Aperture     string   `json:"aperture"`
	ShutterSpeed string   `json:"shutterSpeed"`
	FocalLength  string   `json:"focalLength"`
	DateTaken    string   `json:"dateTaken,omitempty"`
	Orientation  int      `json:"orientation"`
	ColorSpace   string   `json:"colorSpace"`
	Software     string   `json:"software"`
	GPSLatitude  *float64 `json:"gpsLatitude,omitempty"`
	GPSLongitude *float64 `json:"gpsLongitude,omitempty"`
}

// GPSRequest is the JSON request for PUT /exif/gps
type GPSRequest struct {
	Path      string  `json:"path" binding:"required"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	BackupDir string  `json:"backupDir" binding:"required"`
}

// GPSBatchRequest is the JSON request for PUT /exif/gps/batch
type GPSBatchRequest struct {
	Items     []GPSBatchItem `json:"items" binding:"required"`
	BackupDir string         `json:"backupDir" binding:"required"`
}

// GPSBatchItem represents a single item in a batch GPS request
type GPSBatchItem struct {
	Path      string  `json:"path" binding:"required"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// GPSResponse is the JSON response for PUT /exif/gps
type GPSResponse struct {
	Success   bool    `json:"success"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// GPSBatchResponse is the JSON response for PUT /exif/gps/batch
type GPSBatchResponse struct {
	Success int      `json:"success"`
	Failed  int      `json:"failed"`
	FailedFiles []string `json:"failedFiles,omitempty"`
}

// LocationCandidate represents a suggested location
type LocationCandidate struct {
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	NameLocal  string  `json:"nameLocal"`
	NameEng    string  `json:"nameEng"`
	PhotoCount int     `json:"photoCount"`
}

// LocationCandidatesResponse is the JSON response for GET /exif/location-candidates
type LocationCandidatesResponse struct {
	Candidates []LocationCandidate `json:"candidates"`
}

// ErrorResponse is a generic error response
type ErrorResponse struct {
	Error string `json:"error"`
}
