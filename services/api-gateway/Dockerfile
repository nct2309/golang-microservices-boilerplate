FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Find the main.go file and build it
# Build using the correct path to main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o api-gateway ./services/api-gateway/cmd/main.go

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/api-gateway .
COPY --from=builder /app/services/api-gateway/.env .
COPY --from=builder /app/swagger ./swagger
COPY --from=builder /app/proto ./proto

ENV $(cat .env | xargs)

CMD ["./api-gateway"]