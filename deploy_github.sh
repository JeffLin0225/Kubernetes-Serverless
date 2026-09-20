#!/usr/bin/env bash
# ============================================================
# SEP (Serverless Execution Platform) - GHCR 部署腳本
# 用法:
#   ./deploy_github.sh [stg|prod] [sha]
#   例如:
#     ./deploy_github.sh stg                                    # 自動抓 origin/stg 最新 commit 的 image
#     ./deploy_github.sh stg 7d8e7d7e8a33fa60abaafc17d2fe1fd63d56a15e  # 指定特定 SHA 版本
#
# 職責：從 GHCR 拉取 CI（CI-Build.yaml）已建置好的 Image，部署到本機 K8s 叢集
#       不會 docker build 任何東西，也不會有任何雲端 CD 連進本機
#       （本機叢集僅能手動觸發部署，見 deploy.sh 內的說明）
#
# 注意：dev 環境沒有對應的 CI 建置分支，請改用 ./deploy.sh dev --build
# ============================================================

set -e

ENV=${1:-stg}
SHA=$2

REGISTRY="ghcr.io/jefflin0225"
NAMESPACE="ns-sep-${ENV}"
ENGINE_CONFIGMAP_NAME="sep-engine-cm"
ENGINE_ENV_FILE="services/engine/.env.${ENV}"
CLEANER_CONFIGMAP_NAME="sep-cleaner-cm"
CLEANER_ENV_FILE="services/cleaner/.env.${ENV}"

# CI-Build.yaml 只有 stg / main 兩個分支選項，prod 環境對應 main 分支的 image
case "$ENV" in
  stg)  BRANCH="stg" ;;
  prod) BRANCH="main" ;;
  *)
    echo "❌ [Error] 此腳本僅支援 stg / prod（對應 CI-Build.yaml 的 stg / main 分支）"
    echo "本地開發請改用: ./deploy.sh dev --build"
    exit 1
    ;;
esac

echo "============================================================"
echo "🚀 [SEP Deploy via GHCR] 目標環境: ${ENV} | 目標 Namespace: ${NAMESPACE}"
echo "============================================================"

# 1. 檢查對應環境的 .env 檔案是否存在
if [ ! -f "$ENGINE_ENV_FILE" ]; then
  echo "❌ [Error] 找不到環境設定檔: ${ENGINE_ENV_FILE}"
  echo "請確認 services/engine/ 下是否有對應的 .env.${ENV}"
  exit 1
fi

# ============================================================
# [Pull 階段] 從 GHCR 拉取 CI 已建置好的 Image（Immutable Tag: Git SHA）
# ============================================================

if [ -z "$SHA" ]; then
  echo "🔍 [1/4] 未指定 SHA，自動抓取 origin/${BRANCH} 最新 commit..."
  git fetch origin "${BRANCH}" --quiet
  SHA=$(git rev-parse "origin/${BRANCH}")
fi
echo "📌 使用 Image Tag (Git SHA): ${SHA}"

ENGINE_IMAGE="${REGISTRY}/${BRANCH}/sep-engine:${SHA}"
CLEANER_IMAGE="${REGISTRY}/${BRANCH}/sep-cleaner:${SHA}"

echo "🐳 [2/4] 正在從 GHCR 拉取 Image..."
docker pull "${ENGINE_IMAGE}"
docker pull "${CLEANER_IMAGE}"
echo "✅ Image 拉取完成"

# ============================================================
# [CD 階段] 部署至本機 Kubernetes 叢集（手動觸發，非雲端自動 CD）
# ============================================================

echo "☸️  [3/4] 確認 Namespace '${NAMESPACE}' 存在..."
if ! kubectl get namespace "${NAMESPACE}" &>/dev/null; then
  echo "❌ [Error] Namespace '${NAMESPACE}' 不存在！"
  echo "請先執行: kubectl create namespace ${NAMESPACE}"
  exit 1
fi
echo "✅ Namespace '${NAMESPACE}' 確認存在"

echo "📦 同步 ${ENGINE_ENV_FILE} / ${CLEANER_ENV_FILE} 至 ConfigMap..."
kubectl create configmap "${ENGINE_CONFIGMAP_NAME}" \
  --from-file=.env="${ENGINE_ENV_FILE}" \
  -n "${NAMESPACE}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create configmap "${CLEANER_CONFIGMAP_NAME}" \
  --from-file=.env="${CLEANER_ENV_FILE}" \
  -n "${NAMESPACE}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "⚙️  [4/4] 執行 Helm 一鍵安裝 / 更新（套用 GHCR Image，覆蓋 values.yaml 的本地 image 設定）..."
helm upgrade --install sep ./charts \
  -f ./charts/values.yaml \
  -f ./charts/${ENV}/values.yaml \
  --set engine.image.repository="${REGISTRY}/${BRANCH}/sep-engine" \
  --set engine.image.tag="${SHA}" \
  --set engine.image.pullPolicy="IfNotPresent" \
  --set cleaner.image.repository="${REGISTRY}/${BRANCH}/sep-cleaner" \
  --set cleaner.image.tag="${SHA}" \
  --set cleaner.image.pullPolicy="IfNotPresent" \
  -n "${NAMESPACE}"

echo "🔄 [CD Reload] 正在觸發 Deployment 滾動重啟以套用最新設定..."
kubectl rollout restart deployment/sep-engine deployment/sep-cleaner -n "${NAMESPACE}" 2>/dev/null || true
kubectl rollout status deployment/sep-engine -n "${NAMESPACE}" --timeout=60s || true
kubectl rollout status deployment/sep-cleaner -n "${NAMESPACE}" --timeout=60s || true

echo "============================================================"
echo "🎉 部署完成！查看當前 Pod 運行狀態："
echo "============================================================"
kubectl get pods -n "${NAMESPACE}"
