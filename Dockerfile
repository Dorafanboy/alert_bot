FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bot ./cmd/bot

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/bot .
COPY .env .env

CMD ["./bot"] 