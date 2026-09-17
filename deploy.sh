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

# 2. （可選）本地 Docker 打包
if [ "$BUILD_FLAG" == "--build" ]; then
  echo "🐳 [1/4] 正在本地打包 Docker Image: sep-engine:${ENV} ..."
  docker build --no-cache -t "sep-engine:${ENV}" -f services/engine/Dockerfile .
  echo "🐳 [1/4] 正在本地打包 Docker Image: sep-cleaner:${ENV} ..."
  docker build --no-cache -t "sep-cleaner:${ENV}" -f services/cleaner/Dockerfile .
  echo "✅ Docker 打包完成！"
else
  echo "⏩ [1/4] 跳過 Docker 打包（若需要重新打包請加參數: ./deploy.sh ${ENV} --build）"
fi

# 3. 確認 Namespace 已存在（需預先建立，部署腳本不負責建立 namespace）
echo "☸️  [2/4] 確認 Namespace '${NAMESPACE}' 存在..."
if ! kubectl get namespace "${NAMESPACE}" &>/dev/null; then
  echo "❌ [Error] Namespace '${NAMESPACE}' 不存在！"
  echo "請先執行: kubectl create namespace ${NAMESPACE}"
  exit 1
fi
echo "✅ Namespace '${NAMESPACE}' 確認存在"

# 4. CI/CD 打包 .env 為 ConfigMap
echo "📦 [3/4] 同步 ${ENGINE_ENV_FILE} 至 ConfigMap '${ENGINE_CONFIGMAP_NAME}' ..."
kubectl create configmap "${ENGINE_CONFIGMAP_NAME}" \
  --from-file=.env="${ENGINE_ENV_FILE}" \
  -n "${NAMESPACE}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create configmap "${CLEANER_CONFIGMAP_NAME}" \
  --from-file=.env="${CLEANER_ENV_FILE}" \
  -n "${NAMESPACE}" \
  --dry-run=client -o yaml | kubectl apply -f -


# 5. 執行 Helm 部署
echo "⚙️  [4/4] 執行 Helm 一鍵安裝 / 更新 ..."
helm upgrade --install sep ./charts \
  -f ./charts/values.yaml \
  -f ./charts/${ENV}/values.yaml \
  -n "${NAMESPACE}"

# 6. 優雅滾動重啟 Pod（確保一定吃到最新的 ConfigMap 與 Image）
echo "🔄 [Reload] 正在觸發 Deployment 滾動重啟以套用最新設定..."
kubectl rollout restart deployment/sep-engine deployment/sep-cleaner -n "${NAMESPACE}" 2>/dev/null || true
kubectl rollout status deployment/sep-engine -n "${NAMESPACE}" --timeout=60s || true
kubectl rollout status deployment/sep-cleaner -n "${NAMESPACE}" --timeout=60s || true

echo "============================================================"
echo "🎉 部署完成！查看當前 Pod 運行狀態："
echo "============================================================"
kubectl get pods -n "${NAMESPACE}"
