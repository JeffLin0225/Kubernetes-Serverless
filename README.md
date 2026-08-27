# Kubernetes Serverless Engine

一個輕量、高擴展的 Kubernetes Serverless 執行引擎與自動監控收割系統，接收來自 Prefect 的任務請求，動態在 K8s 叢集上建立 Job 執行，並提供獨立的死 Pod 自動清理收割微服務。

---

## 架構概覽

```
[ 外部系統 / Prefect Server ]
             │
             │ POST /api/run (可指定 target namespace, command)
             ▼
┌───────────────────────────────┐
│  Engine (Serverless API 引擎) │
└──────────────┬────────────────┘
               │ K8s API
               ▼
┌───────────────────────────────┐     監控與收割異常死 Pod (ErrImageNeverPull 等)
│        Kubernetes Job         │ ◄──────────────────────────────────────────────┐
│  (動態建立，執行完畢 5m 回收)    │                                                │
└───────────────────────────────┘                                 ┌──────────────┴──────────────┐
                                                                  │ Cleaner (死 Pod 收割微服務)  │
                                                                  └─────────────────────────────┘
```

---

## 專案結構

本專案採用 Service-Centric 微服務架構，將 API 觸發引擎與背景監控服務徹底解耦：

```text
├── engine/                       # 【微服務 1：Serverless API 觸發引擎】
│   ├── main.go                   # Engine 啟動入口 (Port 8080)
│   ├── controller/               # HTTP Handlers (health, run)
│   ├── router/                   # Gin 路由設定
│   └── service/                  # K8s Job 建立與配額解析邏輯
│
├── cleaner/                      # 【微服務 2：Pod 異常死鎖收割監控服務】
│   ├── main.go                   # Cleaner 啟動入口 (Graceful Shutdown)
│   └── service/                  # Pod 狀態巡檢、告警與自動清理邏輯
│
├── common/                       # 【跨服務共用層】
│   ├── config/                   # 環境變數與設定檔載入
│   ├── kube/                     # K8s Clientset 連線管理
│   └── model/                    # 共用 DTO 資料模型 (RunRequest/Response)
│
├── test/                         # HTTP 測試檔 (api.http)
├── configMap/                    # K8s 系統配額表 (system-quotas.yaml)
├── go.mod                        # 根目錄統一套件管理
└── .env                          # 環境變數設定檔
```

---

## 事前準備

### 必要環境
- Go 1.21+
- kubectl
- OrbStack（或其他 K8s 叢集）

### 確認 K8s 叢集連線
```bash
kubectl config current-context
# 預期輸出：orbstack
```

### 套用系統配額 ConfigMap
```bash
kubectl apply -f configMap/system-quotas.yaml

# 確認套用成功
kubectl get configmap serverless-system-quotas
```

---

## 本機啟動

### 1. 啟動 Serverless API 引擎（主要服務）
```bash
go run ./engine
```
啟動成功後會看到：
```
[INFO] 非 In-Cluster 環境，改用 ~/.kube/config 連線（本機開發模式）
[Engine] 服務啟動於 Port: 8080
```

### 2. 啟動 Pod Cleaner 異常收割監控服務（背景微服務）
```bash
go run ./cleaner
```
啟動成功後會看到：
```
[Cleaner] 異常 Pod 收割微服務啟動，巡檢週期: 5s, 目標 Namespace: '' (空字串代表全叢集)
```

---

## API 規格

### 1. 健康檢查
```bash
curl http://localhost:8080/health
```

### 2. 建立任務 Job
```bash
curl -s -X POST http://localhost:8080/api/run \
  -H "Content-Type: application/json" \
  -d '{
    "system_id": "crawler",
    "task_id": "flow-run-abc123",
    "namespace": "default",
    "image": "python:3.11-alpine",
    "command": ["python", "flow.py"]
  }'
```

**Request 欄位說明**

| 欄位 | 型別 | 必填 | 說明 |
|------|------|------|------|
| `system_id` | string | ✅ | 系統識別碼，需在 ConfigMap 中有對應配額 |
| `task_id` | string | ✅ | 任務唯一識別碼（Prefect flow_run_id） |
| `namespace` | string | — | 目標 Namespace（若未填則預設為 `default`） |
| `image` | string | ✅ | 要執行的容器 image |
| `command` | array | — | 覆蓋容器預設指令（例如：`["python", "flow.py"]`，沒填則使用 Dockerfile 預設 CMD） |

**Response 範例**
```json
{
  "status": "ok",
  "message": "已成功接收 TaskID:flow-run-abc123",
  "job_name": "job-crawler-1787493682127"
}
```

---

## 本機測試（HTTP Client）

專案內建 IDE 測試檔 `test/api.http`，可直接在 GoLand / VS Code 中發送請求測試：
* 健康檢查 (`GET /health`)
* 成功建立任務 (`POST /api/run`)
* 異常情況測試（未註冊系統、欄位缺失等）

---

## 系統配額管理

配額設定在 `configMap/system-quotas.yaml`，每個系統需定義 4 個 key：

```yaml
# 格式：<system_id>.<resource>
crawler.cpu_request: "250m"
crawler.memory_request: "512Mi"
crawler.cpu_limit: "500m"
crawler.memory_limit: "1Gi"
```

目前支援的系統：`crawler` / `reporting` / `analytics`。新增系統時在 yaml 加入對應配額後重新 `kubectl apply` 即可。

---

## 常用維運指令

```bash
# 查看所有 ConfigMap
kubectl get configmap

# 查看目前由 Serverless Engine 管理的 Job 與 Pod
kubectl get jobs -l app.kubernetes.io/managed-by=serverless-engine
kubectl get pods -l app.kubernetes.io/managed-by=serverless-engine

# 一鍵清除所有 Engine 管理的 Job
kubectl delete jobs -l app.kubernetes.io/managed-by=serverless-engine

# 查看 Pod 詳細資訊與 Log
kubectl describe pod <pod-name>
kubectl logs <pod-name>
```

---

## 環境變數

| 變數 | 說明 | 預設 |
|------|------|------|
| `NAMESPACE` | 管理配額 ConfigMap 所在的 Namespace | default |
| `PORT` | 服務監聽 Port | 8080 |
| `IMAGE_PULL_POLICY` | Image 拉取策略（Always / IfNotPresent / Never） | Never |
| `SYSTEM_QUOTAS_CONFIGMAP` | 配額 ConfigMap 名稱 | serverless-system-quotas |
| `GIN_MODE` | Gin 模式（debug/release） | debug |
