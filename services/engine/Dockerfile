# 階段 1：編譯環境
FROM golang:1.25-alpine AS builder

WORKDIR /app

# 複製依賴並下載 ( Docker 快取 )
COPY go.mod go.sum ./
RUN go mod download

# 複製整個專案代碼（包含 common/ 與 services/）
COPY . .

# 編譯 Engine 二進制檔案
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/engine-bin ./services/engine

# 階段 2：輕量運行環境（只有 10~15MB） ====
FROM alpine:3.20

WORKDIR /app

# 從第一階段把執行檔複製過來
COPY --from=builder /app/engine-bin /app/engine-bin

# 暴露服務 Port
EXPOSE 8080

# 啟動命令
CMD ["/app/engine-bin"]