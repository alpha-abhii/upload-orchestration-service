# ── Stage 1: Build ────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

# Install git (needed by go mod for some dependencies)
RUN apk add --no-cache git

WORKDIR /app

# Copy dependency files first — Docker caches this layer.
# If only source code changes, this layer is not rebuilt.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server/...

# ── Stage 2: Run ──────────────────────────────────────────────
FROM alpine:3.23

# Add CA certificates — needed for HTTPS calls to AWS
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy only the compiled binary from stage 1
COPY --from=builder /app/server .

EXPOSE 8080

CMD ["./server"]