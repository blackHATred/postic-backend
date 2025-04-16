FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git and build dependencies
RUN apk add --no-cache git

# Copy go.mod and go.sum files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o gateway ./cmd/gateway

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Copy the binary from the builder stage
COPY --from=builder /app/gateway /app/gateway

# Create a non-root user and set ownership
RUN adduser -D appuser && chown -R appuser:appuser /app
USER appuser

# Expose port 80
EXPOSE 80

# Create a directory for storing .env file (optional)
RUN mkdir -p /app/config

# Command to run the application
ENTRYPOINT ["/app/gateway"]