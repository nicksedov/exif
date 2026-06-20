package application

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"exif-service/internal/domain"
)

// trashTimestampFormat is used for backup file naming.
const trashTimestampFormat = "20060102_150405"

// GPSWriter handles GPS coordinate writing to image EXIF metadata.
type GPSWriter struct {
	trashDir string
}

// NewGPSWriter creates a new GPS writer with the given trash directory.
func NewGPSWriter(trashDir string) *GPSWriter {
	return &GPSWriter{trashDir: trashDir}
}

// WriteGPS backs up the original file and writes GPS coordinates to its EXIF metadata.
// Uses a 3-attempt strategy: direct write, strip InteropIFD + retry, nuclear strip + restore.
func (w *GPSWriter) WriteGPS(filePath string, lat, lng float64, meta *domain.ImageMetadata) error {
	// Validate coordinates
	if lat < -90 || lat > 90 {
		return fmt.Errorf("invalid latitude: %f (must be between -90 and 90)", lat)
	}
	if lng < -180 || lng > 180 {
		return fmt.Errorf("invalid longitude: %f (must be between -180 and 180)", lng)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".jpg" && ext != ".jpeg" {
		return fmt.Errorf("GPS can only be written to JPEG files, got: %s", ext)
	}

	backupPath, backupErr := createBackup(filePath, w.trashDir)
	if backupErr != nil {
		return fmt.Errorf("failed to create backup: %w", backupErr)
	}

	latRef := "N"
	lngRef := "E"
	absLat := lat
	absLng := lng
	if lat < 0 {
		latRef = "S"
		absLat = -lat
	}
	if lng < 0 {
		lngRef = "W"
		absLng = -lng
	}

	gpsArgs := func() []string {
		return []string{
			"-overwrite_original", "-m",
			fmt.Sprintf("-GPSLatitude=%.8f", absLat),
			fmt.Sprintf("-GPSLatitudeRef=%s", latRef),
			fmt.Sprintf("-GPSLongitude=%.8f", absLng),
			fmt.Sprintf("-GPSLongitudeRef=%s", lngRef),
			filePath,
		}
	}

	// Attempt 1: direct write with -m
	cmd := exec.Command("exiftool", gpsArgs()...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		log.Printf("EXIF WriteGPS: GPS written to %s (lat=%.8f, lng=%.8f)", filepath.Base(filePath), lat, lng)
		return nil
	}
	log.Printf("EXIF WriteGPS: attempt 1 failed for %s: %v, output: %s", filepath.Base(filePath), err, string(output))

	// Attempt 2: strip corrupted InteropIFD first, then retry
	stripCmd := exec.Command("exiftool", "-overwrite_original", "-m", "-InteropIFD:all=", filePath)
	if stripOut, stripErr := stripCmd.CombinedOutput(); stripErr != nil {
		log.Printf("EXIF WriteGPS: InteropIFD strip failed for %s: %v, output: %s",
			filepath.Base(filePath), stripErr, string(stripOut))
	}

	cmd = exec.Command("exiftool", gpsArgs()...)
	output, err = cmd.CombinedOutput()
	if err == nil {
		log.Printf("EXIF WriteGPS: GPS written to %s after InteropIFD strip (lat=%.8f, lng=%.8f)", filepath.Base(filePath), lat, lng)
		return nil
	}
	log.Printf("EXIF WriteGPS: attempt 2 failed for %s: %v, output: %s", filepath.Base(filePath), err, string(output))

	// Attempt 3: strip ALL EXIF, then restore from backup + GPS + DB overrides
	nukeCmd := exec.Command("exiftool", "-overwrite_original", "-m", "-exif:all=", filePath)
	if nukeOut, nukeErr := nukeCmd.CombinedOutput(); nukeErr != nil {
		log.Printf("EXIF WriteGPS: EXIF strip failed for %s: %v, output: %s",
			filepath.Base(filePath), nukeErr, string(nukeOut))
		return fmt.Errorf("exiftool failed to write GPS (all attempts): %w", err)
	}

	restoreArgs := []string{"-overwrite_original", "-m", "-tagsFromFile", backupPath, "-all:all"}
	restoreArgs = append(restoreArgs, gpsArgs()[2:len(gpsArgs())-1]...)
	restoreArgs = append(restoreArgs, metadataRestoreArgs(meta)...)
	restoreArgs = append(restoreArgs, filePath)

	cmd = exec.Command("exiftool", restoreArgs...)
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("EXIF WriteGPS: attempt 3 failed for %s: %v, output: %s", filepath.Base(filePath), err, string(output))
		return fmt.Errorf("exiftool failed to write GPS (all attempts): %w", err)
	}

	log.Printf("EXIF WriteGPS: GPS + full metadata restored from backup for %s (lat=%.8f, lng=%.8f)", filepath.Base(filePath), lat, lng)
	return nil
}

// WriteExifField writes an arbitrary EXIF tag value to a file.
func WriteExifField(filePath, tag, value string) error {
	cmd := exec.Command("exiftool", "-overwrite_original", "-m",
		fmt.Sprintf("-%s=%s", tag, value), filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exiftool failed to write %s: %v, output: %s", tag, err, string(output))
	}
	return nil
}

// StripExif removes specified EXIF tags from a file. If tags is nil/empty, removes all EXIF.
func StripExif(filePath string, tags []string) error {
	args := []string{"-overwrite_original", "-m"}
	if len(tags) == 0 {
		args = append(args, "-exif:all=")
	} else {
		for _, tag := range tags {
			args = append(args, fmt.Sprintf("-%s=", tag))
		}
	}
	args = append(args, filePath)

	cmd := exec.Command("exiftool", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exiftool strip failed: %v, output: %s", err, string(output))
	}
	return nil
}

// CopyExif copies EXIF data from source to target file.
func CopyExif(sourcePath, targetPath string, tags []string) error {
	args := []string{"-overwrite_original", "-m", "-tagsFromFile", sourcePath}
	if len(tags) == 0 {
		args = append(args, "-all:all")
	} else {
		for _, tag := range tags {
			args = append(args, fmt.Sprintf("-%s", tag))
		}
	}
	args = append(args, targetPath)

	cmd := exec.Command("exiftool", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exiftool copy failed: %v, output: %s", err, string(output))
	}
	return nil
}

// metadataRestoreArgs builds exiftool arguments to restore DB-stored EXIF fields.
func metadataRestoreArgs(meta *domain.ImageMetadata) []string {
	if meta == nil {
		return nil
	}
	var args []string
	if meta.CameraModel != "" {
		args = append(args, fmt.Sprintf("-Model=%s", meta.CameraModel))
	}
	if meta.LensModel != "" {
		args = append(args, fmt.Sprintf("-LensModel=%s", meta.LensModel))
	}
	if meta.ISO != 0 {
		args = append(args, fmt.Sprintf("-ISO#=%d", meta.ISO))
	}
	if meta.Aperture != "" {
		val := strings.TrimPrefix(meta.Aperture, "f/")
		args = append(args, fmt.Sprintf("-FNumber=%s", val))
	}
	if meta.ShutterSpeed != "" {
		val := strings.TrimSuffix(meta.ShutterSpeed, "s")
		args = append(args, fmt.Sprintf("-ExposureTime=%s", val))
	}
	if meta.FocalLength != "" {
		val := strings.TrimSuffix(meta.FocalLength, "mm")
		args = append(args, fmt.Sprintf("-FocalLength=%s", val))
	}
	if meta.DateTaken != nil {
		args = append(args, fmt.Sprintf("-DateTimeOriginal=%s", meta.DateTaken.Format("2006:01:02 15:04:05")))
	}
	if meta.Orientation != 0 {
		args = append(args, fmt.Sprintf("-Orientation#=%d", meta.Orientation))
	}
	if meta.ColorSpace != "" {
		args = append(args, fmt.Sprintf("-ColorSpace=%s", meta.ColorSpace))
	}
	if meta.Software != "" {
		args = append(args, fmt.Sprintf("-Software=%s", meta.Software))
	}
	return args
}

// createBackup copies the original file to a backup location before EXIF modification.
func createBackup(filePath, trashDir string) (string, error) {
	dir := filepath.Dir(filePath)
	if trashDir != "" {
		dir = trashDir
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create trash directory: %w", err)
		}
	}

	ext := filepath.Ext(filePath)
	nameWithoutExt := strings.TrimSuffix(filepath.Base(filePath), ext)
	backupName := fmt.Sprintf("%s_backup_%s%s", nameWithoutExt, time.Now().Format(trashTimestampFormat), ext)
	backupPath := filepath.Join(dir, backupName)

	src, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	log.Printf("EXIF WriteGPS: backup created at %s", backupPath)
	return backupPath, nil
}
