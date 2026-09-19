# ArgoCD 安裝與操作指南

## 一、安裝 ArgoCD（在 OrbStack K8s 上）

### 1. 建立 Namespace
```bash
kubectl apply -f argocd/namespace.yaml
```

### 2. 安裝 ArgoCD
```bash
kubectl apply -n argocd \
  -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

### 3. 等待所有 Pod 啟動完成
```bash
kubectl wait --for=condition=Ready pods --all -n argocd --timeout=120s
```

### 4. 取得 ArgoCD 管理員初始密碼
```bash
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d && echo
```
> 帳號: `admin`

---

## 二、開啟 ArgoCD Web UI

```bash
# Port-forward（瀏覽器開啟 https://localhost:8443）
kubectl port-forward svc/argocd-server -n argocd 8443:443
```

開啟瀏覽器：`https://localhost:8443`
- 帳號：`admin`
- 密碼：上一步取得的密碼
- ⚠️ 瀏覽器會提示不安全（自簽憑證），點「進階」→「繼續前往」即可

---

## 三、設定 GHCR Image Pull Secret（PROD Namespace）

```bash
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=JeffLin0225 \
  --docker-password=<YOUR_GITHUB_PAT> \
  -n ns-sep-prod
```

> GitHub PAT 產生方式：
> GitHub → Settings → Developer settings → Personal access tokens → Tokens (classic)
> → Generate new token → 勾選 `read:packages`

---

## 四、部署 ArgoCD Application

```bash
# 1. 建立 AppProject（權限隔離）
kubectl apply -f argocd/project.yaml

# 2. 建立 Application（自動同步 PROD）
kubectl apply -f argocd/application.yaml
```

---

## 五、驗證

### 在 ArgoCD UI 中確認
1. 開啟 `https://localhost:8443`
2. 應該會看到 `sep-prod` Application
3. 狀態應為 `Synced`（綠色）
4. 點進去可以看到所有 K8s 資源的即時狀態

### 用 CLI 確認
```bash
# 查看 Application 狀態
kubectl get applications -n argocd

# 查看 PROD Pod 運行狀態
kubectl get pods -n ns-sep-prod
```

---

## 六、日常操作

### PROD 部署流程（全自動）
1. 開發者 merge PR 到 `main` branch
2. GitHub Actions CI 自動 Build → Push Image 到 GHCR
3. CI 自動更新 `charts/prod/values.yaml` 的 image tag
4. ArgoCD 偵測到 Git 變動 → 自動 Sync 部署
5. 在 ArgoCD UI 觀察部署狀態 ✅

### 手動觸發同步（如果不想等 ArgoCD 自動偵測）
```bash
# 安裝 ArgoCD CLI（macOS）
brew install argocd

# 登入
argocd login localhost:8443 --username admin --password <PASSWORD> --insecure

# 手動觸發同步
argocd app sync sep-prod
```

### 回滾到前一個版本
```bash
# 在 ArgoCD UI 中點 "History and Rollback" → 選擇要回滾的版本 → Rollback
# 或用 CLI：
argocd app rollback sep-prod
```
