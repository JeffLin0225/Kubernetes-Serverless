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
CONFIGMAP_NAME="sep-engine-env"
ENV_FILE="services/engine/.env.${ENV}"

echo "============================================================"
echo "🚀 [SEP Deploy] 目標環境: ${ENV} | 目標 Namespace: ${NAMESPACE}"
echo "============================================================"

# 1. 檢查對應環境的 .env 檔案是否存在
if [ ! -f "$ENV_FILE" ]; then
  echo "❌ [Error] 找不到環境設定檔: ${ENV_FILE}"
  echo "請確認 services/engine/ 下是否有對應的 .env.${ENV}"
  exit 1
fi

# 2. （可選）本地 Docker 打包
if [ "$BUILD_FLAG" == "--build" ]; then
  echo "🐳 [1/4] 正在本地打包 Docker Image: sep-engine:${ENV} ..."
  docker build -t "sep-engine:${ENV}" -f services/engine/Dockerfile .
  echo "✅ Docker 打包完成！"
else
  echo "⏩ [1/4] 跳過 Docker 打包（若需要重新打包請加參數: ./deploy.sh ${ENV} --build）"
fi

# 3. 確保 Namespace 存在（冪等性，已存在不會報錯）
echo "☸️  [2/4] 確認 Namespace '${NAMESPACE}' ..."
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

# 4. CI/CD 打包 .env 為 ConfigMap
echo "📦 [3/4] 同步 ${ENV_FILE} 至 ConfigMap '${CONFIGMAP_NAME}' ..."
kubectl create configmap "${CONFIGMAP_NAME}" \
  --from-file=.env="${ENV_FILE}" \
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
kubectl rollout restart deployment/sep-engine -n "${NAMESPACE}" 2>/dev/null || true

echo "============================================================"
echo "🎉 部署完成！查看當前 Pod 運行狀態："
echo "============================================================"
kubectl get pods -n "${NAMESPACE}"
