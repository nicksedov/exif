package domain

import "time"

// ImageFile represents an image file in the database
type ImageFile struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Path      string    `gorm:"uniqueIndex;not null" json:"path"`
	Size      int64     `gorm:"not null;index:idx_size_hash" json:"size"`
	Hash      string    `gorm:"not null;index:idx_size_hash" json:"hash"`
	ModTime   time.Time `gorm:"not null" json:"modTime"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ImageMetadata stores extracted EXIF metadata for an image.
// Geolocation is resolved via GeolocationRef -> GeolocationCache (Nominatim-backed).
type ImageMetadata struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	ImageFileID    uint       `gorm:"uniqueIndex;not null" json:"imageFileId"`
	Width          int        `json:"width"`
	Height         int        `json:"height"`
	CameraModel    string     `json:"cameraModel"`
	LensModel      string     `json:"lensModel"`
	ISO            int        `json:"iso"`
	Aperture       string     `json:"aperture"`
	ShutterSpeed   string     `json:"shutterSpeed"`
	FocalLength    string     `json:"focalLength"`
	DateTaken      *time.Time `json:"dateTaken"`
	Orientation    int        `json:"orientation"`
	ColorSpace     string     `json:"colorSpace"`
	Software       string     `json:"software"`
	GeolocationRef *uint      `gorm:"index" json:"geolocationRef"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// GeolocationCache stores reverse-geocoded location names for unique GPS coordinate pairs.
type GeolocationCache struct {
	ID           uint    `gorm:"primaryKey" json:"id"`
	GPSLatitude  float64 `gorm:"uniqueIndex:idx_geo_lat_lng;not null" json:"gpsLatitude"`
	GPSLongitude float64 `gorm:"uniqueIndex:idx_geo_lat_lng;not null" json:"gpsLongitude"`
	NameLocal    string  `gorm:"type:text" json:"nameLocal"`
	NameEng      string  `gorm:"type:text" json:"nameEng"`
}
