FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build both gateways
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/api-gateway ./cmd/api-gateway
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/ws-gateway ./cmd/ws-gateway

# --- API Gateway image ---
FROM alpine:3.21 AS api-gateway
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/api-gateway /usr/local/bin/api-gateway
EXPOSE 8080
CMD ["api-gateway"]

# --- WS Gateway image ---
FROM alpine:3.21 AS ws-gateway
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/ws-gateway /usr/local/bin/ws-gateway
EXPOSE 8081
CMD ["ws-gateway"]
