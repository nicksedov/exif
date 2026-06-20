# EXIF Microservice

A Go-based microservice for reading and writing EXIF metadata in image files. Provides a REST API and an [MCP](https://modelcontextprotocol.io/) endpoint for AI-assisted workflows. Uses [ExifTool](https://exiftool.org/) under the hood and stores image metadata in a PostgreSQL database.

## Features

- **Read EXIF metadata** — extract camera model, lens, ISO, aperture, shutter speed, focal length, GPS coordinates, and more
- **Write GPS coordinates** — update a single image or batch-update multiple files
- **Missing metadata detection** — find images that lack date-taken or GPS data (paginated)
- **Location candidates** — suggest GPS locations from same-day photos that already have coordinates
- **MCP endpoint** — expose EXIF tools via the Model Context Protocol for AI agent integration
- **Health check** — service status including exiftool availability and database connectivity

## Tech Stack

| Layer        | Technology                        |
| ------------ | --------------------------------- |
| Language     | Go 1.25                           |
| Web framework| Gin                               |
| Database     | PostgreSQL (via GORM)             |
| EXIF engine  | ExifTool (perl-image-exiftool)    |
| MCP          | modelcontextprotocol/go-sdk       |
| Container    | Docker (multi-stage Alpine build) |

## Project Structure

```
cmd/server/          — application entry point
internal/
  application/       — business logic (EXIF extraction, GPS writing)
  domain/            — domain models
  infrastructure/
    config/          — environment-based configuration
    database/        — PostgreSQL connection setup
  interfaces/
    dto/             — request/response DTOs
    handler/         — HTTP handlers and router
mcp/                 — MCP server and tool definitions
docs/                — OpenAPI spec and MCP tool documentation
```

## Getting Started

### Prerequisites

- Go 1.25+
- PostgreSQL
- ExifTool (installed via `perl-image-exiftool` on Alpine, or download from [exiftool.org](https://exiftool.org/))

### Environment Variables

Set the following environment variables before starting the service:

| Variable      | Default        | Description                   |
| ------------- | -------------- | ----------------------------- |
| `DB_HOST`     | `localhost`    | PostgreSQL host               |
| `DB_PORT`     | `5432`         | PostgreSQL port               |
| `DB_USER`     | `postgres`     | Database user                 |
| `DB_PASSWORD` | `postgres`     | Database password             |
| `DB_NAME`     | `image_toolkit`| Database name                 |
| `DB_SSLMODE`  | `disable`      | SSL mode                      |
| `SERVER_HOST` | `0.0.0.0`     | Server bind address           |
| `SERVER_PORT` | `5171`         | Server port                   |
| `TRASH_DIR`   | `./trash`      | Directory for trashed files   |
| `LOG_LEVEL`   | `info`         | Logging level                 |

### Run Locally

```bash
# Install dependencies
go mod download

# Run the service
go run ./cmd/server/
```

### Run with Docker

```bash
# Build the image
docker build -t exif-service .

# Run the container
docker run -p 5171:5171 \
  -e DB_HOST=host.docker.internal \
  -e DB_PORT=5432 \
  -e DB_USER=postgres \
  -e DB_PASSWORD=postgres \
  -e DB_NAME=image_toolkit \
  exif-service
```

## API Endpoints

All routes are prefixed with `/exif`.

| Method | Path                     | Description                                  |
| ------ | ------------------------ | -------------------------------------------- |
| GET    | `/health`                | Service health check                         |
| GET    | `/metadata?path=`        | Read EXIF metadata from a file               |
| GET    | `/missing`               | List images missing date or GPS (paginated)  |
| PUT    | `/gps`                   | Write GPS coordinates to a single image      |
| PUT    | `/gps/batch`             | Batch-write GPS coordinates to multiple files|
| GET    | `/location-candidates`   | Suggest locations from same-day photos       |
| ANY    | `/mcp`                   | MCP endpoint for AI agent integration        |

See [docs/openapi.yaml](docs/openapi.yaml) for the full OpenAPI specification.

## License

Private — all rights reserved.
