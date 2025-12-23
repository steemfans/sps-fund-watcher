# Build stage for Go services
FROM golang:1.23-alpine3.19 AS go-builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build sync service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sync ./cmd/sync

# Build API service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api ./cmd/api

# Build compensator tool
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o compensator ./cmd/compensator

# Build stage for frontend
FROM node:20-alpine3.19 AS frontend-builder

# Install pnpm using npm (more stable than corepack)
RUN npm install -g pnpm

WORKDIR /build

# Copy package files
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

# Copy source code
COPY web/ .

# Build frontend
RUN pnpm run build

# Final stage
FROM alpine:3.19

# Install dependencies
RUN apk add --no-cache \
    nginx \
    supervisor \
    ca-certificates \
    tzdata

# Create app directory
WORKDIR /app

# Copy Go binaries from builder
COPY --from=go-builder /build/sync /app/sync
COPY --from=go-builder /build/api /app/api
COPY --from=go-builder /build/compensator /app/compensator

# Copy frontend build from builder
COPY --from=frontend-builder /build/dist /app/web/dist

# Copy configuration files
COPY configs/ /app/configs/

# Create nginx directories
RUN mkdir -p /var/log/nginx /var/run/nginx && \
    chown -R nginx:nginx /var/log/nginx /var/run/nginx

# Expose port
EXPOSE 80

# Start supervisord
CMD ["/usr/bin/supervisord", "-c", "/app/configs/supervisord.conf"]

