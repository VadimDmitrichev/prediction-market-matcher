# Stage 1: Build binary
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Cache dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and compile statically linked binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /build/matcher-server ./cmd/grpc_server/main.go

# Stage 2: Minimal production runtime
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata && \
    adduser -D -g '' appuser

WORKDIR /app

COPY --from=builder /build/matcher-server /app/matcher-server
# Copy config if present in repository root
COPY config*.yaml /app/

USER appuser

ENV CONFIG_PATH=/app/config.yaml
EXPOSE 50051

ENTRYPOINT ["/app/matcher-server"]