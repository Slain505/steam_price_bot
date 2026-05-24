# Stage 1: build
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o steam-price-bot ./cmd/bot

# Stage 2: minimal runtime image
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/steam-price-bot .

VOLUME /data
ENV DB_PATH=/data/prices.db

CMD ["./steam-price-bot"]
