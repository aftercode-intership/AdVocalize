# backend/Dockerfile
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api .


# Final stage
FROM alpine:3.18

WORKDIR /root/

# Install runtime dependencies
RUN apk --no-cache add ca-certificates curl

# Copy binary from builder
COPY --from=builder /app/api .

# Expose port
EXPOSE 8081

# Health check
HEALTHCHECK --interval=10s --timeout=5s --retries=5 \
  CMD curl -f http://localhost:8081/health || exit 1

# Run
CMD ["./api"]