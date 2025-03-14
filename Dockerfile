# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o web-analyzer-api ./cmd/api

# Runtime stage
FROM alpine:3.18

# Install CA certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/web-analyzer-api .

# Create a non-root user and switch to it
RUN adduser -D -g '' appuser
USER appuser

# Expose the application port
EXPOSE 9090

# Run the application
CMD ["./web-analyzer-api"]