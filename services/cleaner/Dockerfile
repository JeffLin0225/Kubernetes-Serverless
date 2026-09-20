# --platform=$BUILDPLATFORM：永遠用 runner 原生架構跑編譯器，改用 Go 原生跨平台編譯
# 而不是靠 QEMU 模擬整個 arm64 環境（避免 go build 在模擬 CPU 下慢 5~20 倍）
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /app/cleaner-bin ./services/cleaner

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/cleaner-bin /app/cleaner-bin

CMD ["/app/cleaner-bin"]