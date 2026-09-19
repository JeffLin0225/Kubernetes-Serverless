#!/usr/bin/env bash
# ============================================================
# SEP (Serverless Execution Platform) - 本地 CD 部署腳本
#
# ⚡ 此腳本僅負責 CD（部署），CI（Build & Push）由 GitHub Actions 處理
#
# 用法:
#   ./deploy.sh <env> <image-tag>
#   例如:
#     ./deploy.sh stg sha-abc1234     # 部署指定版本到 STG
#     ./deploy.sh stg                 # 使用 values.yaml 中的預設 tag
#
# 前置需求:
#   1. OrbStack K8s 正在運行
#   2. 已建立 GHCR Image Pull Secret:
#      kubectl create secret docker-registry ghcr-secret \
#        --docker-server=ghcr.io \
#        --docker-username=JeffLin0225 \
#        --docker-password=<YOUR_GITHUB_PAT> \
#        -n ns-sep-stg
# ============================================================

set -e

ENV=${1:-stg}
IMAGE_TAG=${2:-""}

NAMESPACE="ns-sep-${ENV}"
ENGINE_CONFIGMAP_NAME="sep-engine-cm"
ENGINE_ENV_FILE="services/engine/.env.${ENV}"

CLEANER_CONFIGMAP_NAME="sep-cleaner-cm"
CLEANER_ENV_FILE="services/cleaner/.env.${ENV}"

echo "============================================================"
echo "🚀 [SEP CD] 目標環境: ${ENV} | 目標 Namespace: ${NAMESPACE}"
if [ -n "$IMAGE_TAG" ]; then
  echo "🏷️  [SEP CD] Image Tag: ${IMAGE_TAG}"
fi
echo "============================================================"

# 1. 檢查對應環境的 .env 檔案是否存在
if [ ! -f "$ENGINE_ENV_FILE" ]; then
  echo "❌ [Error] 找不到環境設定檔: ${ENGINE_ENV_FILE}"
  echo "請確認 services/engine/ 下是否有對應的 .env.${ENV}"
  exit 1
fi

# ============================================================
# [CD 階段] Continuous Deployment - 部署至本地 OrbStack K8s
# 順序：前置驗證 → 同步設定 ConfigMap → Helm 部署 → 滾動重啟確認
# ============================================================

# CD 前置：確認 Namespace 已存在
echo "☸️  [CD 1/3] 確認 Namespace '${NAMESPACE}' 存在..."
if ! kubectl get namespace "${NAMESPACE}" &>/dev/null; then
  echo "❌ [Error] Namespace '${NAMESPACE}' 不存在！"
  echo "請先執行: kubectl create namespace ${NAMESPACE}"
  exit 1
fi
echo "✅ Namespace '${NAMESPACE}' 確認存在"

# CD Step 1：同步 .env 至 ConfigMap
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
echo "⚙️  [CD 3/3] 執行 Helm 一鍵安裝 / 更新 ..."

# 組建 Helm 指令（如果有指定 Image Tag，則用 --set 覆蓋）
HELM_CMD="helm upgrade --install sep ./charts \
  -f ./charts/values.yaml \
  -f ./charts/${ENV}/values.yaml \
  -n ${NAMESPACE}"

if [ -n "$IMAGE_TAG" ]; then
  HELM_CMD="${HELM_CMD} \
    --set engine.image.tag=${IMAGE_TAG} \
    --set cleaner.image.tag=${IMAGE_TAG}"
  echo "🏷️  使用指定 Image Tag: ${IMAGE_TAG}"
fi

eval $HELM_CMD

# CD Step 3：優雅滾動重啟 Pod
echo "🔄 [CD Reload] 正在觸發 Deployment 滾動重啟以套用最新設定..."
kubectl rollout restart deployment/sep-engine deployment/sep-cleaner -n "${NAMESPACE}" 2>/dev/null || true
kubectl rollout status deployment/sep-engine -n "${NAMESPACE}" --timeout=60s || true
kubectl rollout status deployment/sep-cleaner -n "${NAMESPACE}" --timeout=60s || true

echo "============================================================"
echo "🎉 部署完成！查看當前 Pod 運行狀態："
echo "============================================================"
kubectl get pods -n "${NAMESPACE}"
