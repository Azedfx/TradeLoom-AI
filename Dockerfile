# Stage 1: Build
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# # Generate swagger docs
# RUN which swag || go install github.com/swaggo/swag/cmd/swag@v1.16.4
# RUN swag init -g cmd/main.go -d ./,./docs/sources -o ./docs --parseDependency --parseInternal


# Build the Go binary
RUN go build -o main ./cmd/api

# Stage 2: Run
FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/main .

# # # Copy the templates folder into the container
# COPY --from=builder /app/internal/templates ./internal/templates

# # Copy swagger docs into the container
# COPY --from=builder /app/docs ./docs

EXPOSE 8008

# Run the binary
CMD ["./main"]
