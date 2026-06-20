# EXIF MCP Tools Documentation

The EXIF microservice exposes MCP tools at `POST /exif/mcp` using Streamable HTTP transport.

## Read Tools

### read_exif
Read all EXIF fields from image file.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Absolute path to the image file |

**Returns:** Camera model, lens model, ISO, aperture, shutter speed, focal length, date taken, orientation, color space, software.

### read_gps
Read GPS coordinates from image EXIF.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Absolute path to the image file |

**Returns:** Latitude, longitude (and altitude if available).

### read_all_metadata
Read complete EXIF tag dump (all tags, raw values).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Absolute path to the image file |

**Returns:** All EXIF tags as key-value pairs.

## Write Tools

### write_gps
Write GPS coordinates to image EXIF using 3-attempt strategy with backup.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Absolute path to the image file |
| `latitude` | number | yes | GPS latitude (-90 to 90) |
| `longitude` | number | yes | GPS longitude (-180 to 180) |

### write_exif_field
Write arbitrary EXIF tag value.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Absolute path to the image file |
| `tag` | string | yes | EXIF tag name (e.g., DateTimeOriginal) |
| `value` | string | yes | Value to write |

### strip_exif
Remove specified EXIF tags (or all if tags omitted).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Absolute path to the image file |
| `tags` | string[] | no | List of EXIF tags to remove |

### copy_exif
Copy EXIF data from source to target file.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `source_path` | string | yes | Source file path |
| `target_path` | string | yes | Target file path |
| `tags` | string[] | no | Specific tags to copy (omit for all) |

## Utility Tools

### compare_exif
Compare EXIF metadata between two images, return differences.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path1` | string | yes | Path to first image |
| `path2` | string | yes | Path to second image |

### validate_exif
Validate EXIF integrity (check for corruption, InteropIFD issues).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Absolute path to the image file |
