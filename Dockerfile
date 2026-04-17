# Build stage
FROM golang:1.26-alpine AS builder

# Build arguments for versioning
ARG VERSION=dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build with version information
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -extldflags -static -X main.Version=${VERSION}" \
    -o go-root-ceremony

# Runtime stage
FROM scratch

WORKDIR /
COPY --from=builder /app/go-root-ceremony .

USER 65532:65532
ENTRYPOINT ["/go-root-ceremony"]
