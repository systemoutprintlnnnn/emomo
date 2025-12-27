# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git and build tools (gcc needed for CGO/SQLite)
RUN apk add --no-cache git gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the API binary
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o api ./cmd/api

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/api .
COPY --from=builder /app/configs ./configs

# Create data directory
RUN mkdir -p ./data

# Expose port (Hugging Face Spaces uses 7860 by default)
EXPOSE 7860

# Set default port for Hugging Face Spaces
ENV PORT=7860

# Run the binary
CMD ["./api"]

