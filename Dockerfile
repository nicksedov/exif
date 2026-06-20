# Build stage
FROM golang:1.26.4-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /exif-service ./cmd/server/

# Runtime stage
FROM alpine:3.24

RUN apk add --no-cache \
    perl \
    perl-image-exiftool \
    ca-certificates \
    tzdata

WORKDIR /app

COPY --from=builder /exif-service .

EXPOSE 5171

CMD ["./exif-service"]
