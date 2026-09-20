# 階段 1：編譯環境
# --platform=$BUILDPLATFORM：永遠用 runner 原生架構跑編譯器，改用 Go 原生跨平台編譯
# 而不是靠 QEMU 模擬整個 arm64 環境（避免 go build 在模擬 CPU 下慢 5~20 倍）
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

WORKDIR /app

# 複製依賴並下載 ( Docker 快取 )
COPY go.mod go.sum ./
RUN go mod download

# 複製整個專案代碼（包含 common/ 與 services/）
COPY . .

# 編譯 Engine 二進制檔案（依 buildx 傳入的目標架構跨平台編譯）
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /app/engine-bin ./services/engine

# 階段 2：輕量運行環境（只有 10~15MB） ====
FROM alpine:3.20

WORKDIR /app

# 從第一階段把執行檔複製過來
COPY --from=builder /app/engine-bin /app/engine-bin

# 暴露服務 Port
EXPOSE 8080

# 啟動命令
CMD ["/app/engine-bin"]