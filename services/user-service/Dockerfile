FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# This path should match your project structure
RUN CGO_ENABLED=0 GOOS=linux go build -o user-service ./services/user-service/cmd/

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/user-service .
COPY --from=builder /app/services/user-service/.env .

ENV $(cat .env | xargs)

CMD ["./user-service"]