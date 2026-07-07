# Start from the official Golang image as a build stage
FROM golang:1.23-alpine AS builder

# Set working directory
WORKDIR /app

# Install git for Go module fetching
RUN apk add --no-cache git

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application (replace main.go path if your entrypoint differs)
RUN go build -o driftmap-api ./cmd/api/main.go

# Final minimal image
FROM alpine:3.20

# Set working directory
WORKDIR /app

# Copy the built binary from builder stage
COPY --from=builder /app/driftmap-api .

# Expose port (replace 8080 with your actual API port)
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["./driftmap-api"]
