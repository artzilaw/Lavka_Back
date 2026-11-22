# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /lavka-server ./cmd/server \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /lavka-auth ./cmd/auth-service \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /lavka-catalog ./cmd/catalog-service

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app

COPY --from=builder /lavka-server ./lavka-server
COPY --from=builder /lavka-auth ./lavka-auth
COPY --from=builder /lavka-catalog ./lavka-catalog
COPY config ./config
COPY static ./static

EXPOSE 8000 50051 50052 9091 9092

CMD ["./lavka-server", "--config=./config/local.json"]
