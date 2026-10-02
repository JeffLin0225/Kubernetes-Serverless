#!/usr/bin/env bash
# ============================================================
# SEP (Serverless Execution Platform) - 部署自動化腳本
# 用法:
#   ./deploy.sh [stg|dev|prod] [--build]
#   例如:
#     ./deploy.sh stg          # 僅更新 ConfigMap 並執行 Helm 部署
#     ./deploy.sh stg --build  # 包含本地 Docker Image 重新打包
# ============================================================

set -e

ENV=${1:-stg}
BUILD_FLAG=$2

NAMESPACE="ns-sep-${ENV}"
ENGINE_CONFIGMAP_NAME="sep-engine-cm"
ENGINE_ENV_FILE="services/engine/.env.${ENV}"

CLEANER_CONFIGMAP_NAME="sep-cleaner-cm"
CLEANER_ENV_FILE="services/cleaner/.env.${ENV}"

echo "============================================================"
echo "🚀 [SEP Deploy] 目標環境: ${ENV} | 目標 Namespace: ${NAMESPACE}"
echo "============================================================"

# 1. 檢查對應環境的 .env 檔案是否存在
if [ ! -f "$ENGINE_ENV_FILE" ]; then
  echo "❌ [Error] 找不到環境設定檔: ${ENV_FILE}"
  echo "請確認 services/engine/ 下是否有對應的 .env.${ENV}"
  exit 1
fi

# ============================================================
# [CI 階段] Continuous Integration - 建置產出 Artifact
# 職責：將原始碼編譯打包成可部署的 Docker Image
# 注意：CI 階段只負責 Build，不異動任何 K8s 資源
#       Image Tag 應使用 Immutable Tag（如 git-SHA-日期），
#       禁止使用 :latest 等 mutable tag（避免 Race Condition）
# ============================================================

# 2. （可選）本地 Docker 打包
if [ "$BUILD_FLAG" == "--build" ]; then
  echo "🐳 [CI 1/1] 正在本地打包 Docker Image: sep-engine:${ENV} ..."
  docker build --no-cache -t "sep-engine:${ENV}" -f services/engine/Dockerfile .
  echo "🐳 [CI 1/1] 正在本地打包 Docker Image: sep-cleaner:${ENV} ..."
  docker build --no-cache -t "sep-cleaner:${ENV}" -f services/cleaner/Dockerfile .
  echo "✅ [CI] Docker 打包完成！"
else
  echo "⏩ [CI] 跳過 Docker 打包（若需要重新打包請加參數: ./deploy.sh ${ENV} --build）"
fi

# ============================================================
# [CD 階段] Continuous Deployment - 部署至 Kubernetes 叢集
# 職責：將 CI 產出的 Image 部署至目標環境，更新 K8s 資源
# 順序：前置驗證 → 同步設定 ConfigMap → Helm 部署 → 滾動重啟確認
# ============================================================

# CD 前置：確認 Namespace 已存在（需預先建立，部署腳本不負責建立 namespace）
echo "☸️  [CD 1/3] 確認 Namespace '${NAMESPACE}' 存在..."
if ! kubectl get namespace "${NAMESPACE}" &>/dev/null; then
  echo "❌ [Error] Namespace '${NAMESPACE}' 不存在！"
  echo "請先執行: kubectl create namespace ${NAMESPACE}"
  exit 1
fi
echo "✅ Namespace '${NAMESPACE}' 確認存在"

# CD Step 1：同步 .env 至 ConfigMap
# 將各環境的 .env 設定注入 K8s ConfigMap，掛載進 Pod 內供服務讀取
echo "📦 [CD 2/3] 同步 ${ENGINE_ENV_FILE} 至 ConfigMap '${ENGINE_CONFIGMAP_NAME}' ..."
kubectl create configmap "${ENGINE_CONFIGMAP_NAME}" \
  --from-file=.env="${ENGINE_ENV_FILE}" \
  -n "${NAMESPACE}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create configmap "${CLEANER_CONFIGMAP_NAME}" \
  --from-file=.env="${CLEANER_ENV_FILE}" \
  -n "${NAMESPACE}" \
  --dry-run=client -o yaml | kubectl apply -f -

# CD Step 2：執行 Helm 部署
# helm upgrade --install：若 Release 不存在則 install，已存在則 upgrade（冪等操作）
echo "⚙️  [CD 3/3] 執行 Helm 一鍵安裝 / 更新 ..."
helm upgrade --install sep ./charts \
  -f ./charts/values.yaml \
  -f ./charts/${ENV}/values.yaml \
  -n "${NAMESPACE}"

# CD Step 3：優雅滾動重啟 Pod
# 確保 Pod 一定吃到最新的 ConfigMap 與 Image，而不是沿用舊版快取
echo "🔄 [CD Reload] 正在觸發 Deployment 滾動重啟以套用最新設定..."
kubectl rollout restart deployment/sep-engine deployment/sep-cleaner -n "${NAMESPACE}" 2>/dev/null || true
kubectl rollout status deployment/sep-engine -n "${NAMESPACE}" --timeout=60s || true
kubectl rollout status deployment/sep-cleaner -n "${NAMESPACE}" --timeout=60s || true

echo "============================================================"
echo "🎉 部署完成！查看當前 Pod 運行狀態："
echo "============================================================"
kubectl get pods -n "${NAMESPACE}"
