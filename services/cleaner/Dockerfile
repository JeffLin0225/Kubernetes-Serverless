FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/cleaner-bin ./services/cleaner

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/cleaner-bin /app/cleaner-bin

CMD ["/app/cleaner-bin"]