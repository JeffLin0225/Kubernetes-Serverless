# Kubernetes Serverless Engine

一個輕量的 Serverless 執行引擎，接收來自 Prefect 的任務請求，動態在 Kubernetes 叢集上建立 Job 執行。

## 架構概覽

```
Prefect Server
    │
    │ POST /api/run
    ▼
Serverless Engine (Go + Gin)
    │
    │ K8s API
    ▼
Kubernetes Job（動態建立，跑完自動回收）
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

```bash
go run .
```

啟動成功後會看到：
```
[INFO] 非 In-Cluster 環境，改用 ~/.kube/config 連線（本機開發模式）
服務啟動於 Port: 8080
```

---

## API

### 健康檢查
```bash
curl http://localhost:8080/health
```

### 建立任務 Job
```bash
curl -s -X POST http://localhost:8080/api/run \
  -H "Content-Type: application/json" \
  -d '{
    "system_id": "crawler",
    "task_id": "flow-run-abc123",
    "image": "your-image:latest",
    "command": ["python", "flow.py"]
  }'
```

**Request 欄位**

| 欄位 | 型別 | 必填 | 說明 |
|------|------|------|------|
| `system_id` | string | ✅ | 系統識別碼，需在 ConfigMap 中有對應配額 |
| `task_id` | string | ✅ | 任務唯一識別碼（Prefect flow_run_id） |
| `image` | string | ✅ | 要執行的容器 image |
| `command` | array | — | 覆蓋容器預設指令（例如：`["python", "flow.py"]`） |

**Response**
```json
{
  "status": "ok",
  "message": "已成功接收 TaskID:flow-run-abc123",
  "job_name": "job-crawler-1787493682127"
}
```

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

目前支援的系統：`crawler` / `reporting` / `analytics`

新增系統：在 yaml 加入對應配額後重新 `kubectl apply`。

---

## 常用指令

```bash
# 查看所有 ConfigMap
kubectl get configmap

# 查看目前 Job / Pod
kubectl get jobs
kubectl get pods

# 查看 Pod 詳細資訊
kubectl describe pod <pod-name>

# 查看 Pod 執行 log
kubectl logs <pod-name>

# 手動刪除 Pod
kubectl delete pod <pod-name>
```

---

## 環境變數

| 變數 | 說明 | 預設 |
|------|------|------|
| `NAMESPACE` | K8s Namespace | — |
| `PORT` | 服務監聽 Port | — |
| `IMAGE_PULL_POLICY` | Image 拉取策略 | — |
| `SYSTEM_QUOTAS_CONFIGMAP` | 配額 ConfigMap 名稱 | — |
| `GIN_MODE` | Gin 模式（debug/release） | debug |

本機開發透過 `.env` 設定，K8s 環境透過 ConfigMap 注入。