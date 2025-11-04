# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY mdserve.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mdserve mdserve.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /build/mdserve .

# Create a directory for markdown files
RUN mkdir -p /docs

# Expose default port
EXPOSE 8080

# Default command serves /docs on port 8080
ENTRYPOINT ["/app/mdserve"]
CMD ["/docs", "-port", "8080"]
