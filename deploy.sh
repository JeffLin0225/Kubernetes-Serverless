#!/usr/bin/env bash
# ============================================================
# SEP (Serverless Execution Platform) - 部署自動化腳本
# 用法:
#   ./deploy.sh [stg|dev|prod] [--build]
#   例如:
#     ./deploy.sh stg          # 執行 Helm 部署（ConfigMap 由 Helm 管理）
#     ./deploy.sh stg --build  # 包含本地 Docker Image 重新打包
#
# [大規模版] ConfigMap 改由 Helm Template 統一管理
#   設定來源：charts/values.yaml + charts/{ENV}/values.yaml
#   機密值：由 SP 人員手動建立 K8s Secret（參考 secret-ref.txt）
# ============================================================

set -e

ENV=${1:-stg}
BUILD_FLAG=$2

NAMESPACE="ns-sep-${ENV}"

echo "============================================================"
echo "🚀 [SEP Deploy] 目標環境: ${ENV} | 目標 Namespace: ${NAMESPACE}"
echo "============================================================"

# ============================================================
# [CI 階段] Continuous Integration - 建置產出 Artifact
# ============================================================

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
# 順序：前置驗證 → Helm 部署（含 ConfigMap + Deployment）
#
# [大規模版] ConfigMap 由 Helm 統一管理，不再需要獨立的 kubectl create configmap
#   Config 變動只需改 charts/{ENV}/values.yaml，helm upgrade 自動同步
#   Deployment checksum annotation 確保 config 變動時 Pod 自動滾動重啟
# ============================================================

# CD 前置：確認 Namespace 已存在
echo "☸️  [CD 1/2] 確認 Namespace '${NAMESPACE}' 存在..."
if ! kubectl get namespace "${NAMESPACE}" &>/dev/null; then
  echo "❌ [Error] Namespace '${NAMESPACE}' 不存在！"
  echo "請先執行: kubectl create namespace ${NAMESPACE}"
  exit 1
fi
echo "✅ Namespace '${NAMESPACE}' 確認存在"

# CD Step 1：Helm 一鍵部署（ConfigMap + Deployment + Service + RBAC 全部一起）
# --atomic：部署失敗自動 rollback 到上一版
echo "⚙️  [CD 2/2] 執行 Helm 一鍵安裝 / 更新 ..."
helm upgrade --install sep ./charts \
  -f ./charts/values.yaml \
  -f ./charts/${ENV}/values.yaml \
  -n "${NAMESPACE}" \
  --atomic \
  --timeout 120s

# NOTE: 不再需要手動 kubectl rollout restart
# Deployment 的 checksum/config annotation 會在 ConfigMap 內容變動時自動觸發 Pod 滾動重啟

echo "============================================================"
echo "🎉 部署完成！查看當前 Pod 運行狀態："
echo "============================================================"
kubectl get pods -n "${NAMESPACE}"
