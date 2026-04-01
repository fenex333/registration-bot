FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o graduation-bot .

# ---

FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/graduation-bot .

# Copy credentials if bundled (alternatively mount as volume)
# COPY credentials.json .

CMD ["./graduation-bot"]
