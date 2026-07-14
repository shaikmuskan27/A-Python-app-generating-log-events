FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy source code and go.mod
COPY . .

# Download dependencies and generate go.sum
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -o observer .

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/observer .

CMD ["./observer"]
