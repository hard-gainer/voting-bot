FROM golang:1.23.3-alpine AS builder

WORKDIR /app
COPY . .

RUN apk add --no-cache git gcc musl-dev openssl-dev pkgconfig make \
    && go mod tidy \
    && go mod download

RUN CGO_ENABLED=1 GOOS=linux go build -o voting-bot ./cmd/voting-bot

FROM alpine:3.18

RUN apk add --no-cache ca-certificates tzdata openssl libstdc++

WORKDIR /app

COPY --from=builder /app/voting-bot .

COPY --from=builder /app/.env ./.env

RUN chmod 600 ./.env

CMD ["./voting-bot"]