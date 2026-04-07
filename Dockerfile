FROM golang:1.24.3-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY ./cmd ./cmd

# Build
RUN go build -o bot ./cmd/creation_bot

# Application run layer
FROM scratch
COPY --from=builder /app/bot /bot
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

ENTRYPOINT ["/bot"]
