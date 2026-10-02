#!/usr/bin/env bash
# ============================================================
# SEP (Serverless Execution Platform) - 配額 ConfigMap 更新腳本
# 用法:
#   ./scripts/update-quotas.sh [stg|dev|prod]
#   例如:
#     ./scripts/update-quotas.sh stg
#
# 職責：只渲染並套用 sep-system-quotas 這個 ConfigMap（charts/templates/configmap-quotas.yaml）
#       不會 docker build、不會跑 helm upgrade 整個 chart、不會 rollout restart 任何 Deployment
#
# 使用時機：只有新增/調整 charts/values.yaml 或 charts/<env>/values.yaml 裡的 quotas.systems 設定時使用。
#          engine 是在收到請求時即時向 K8s API 讀取這個 ConfigMap（非啟動時載入、非 mount volume），
#          所以更新 ConfigMap 後對下一個請求立刻生效，完全不需要重啟或重新部署。
# ============================================================

set -e

cd "$(dirname "${BASH_SOURCE[0]}")/.."

ENV=${1:-stg}
NAMESPACE="ns-sep-${ENV}"

echo "============================================================"
echo "📦 [Update Quotas] 目標環境: ${ENV} | 目標 Namespace: ${NAMESPACE}"
echo "============================================================"

if ! kubectl get namespace "${NAMESPACE}" &>/dev/null; then
  echo "❌ [Error] Namespace '${NAMESPACE}' 不存在！"
  echo "請先執行: kubectl create namespace ${NAMESPACE}"
  exit 1
fi

if [ ! -f "./charts/${ENV}/values.yaml" ]; then
  echo "❌ [Error] 找不到環境設定檔: ./charts/${ENV}/values.yaml"
  exit 1
fi

echo "⚙️  渲染並套用 sep-system-quotas ConfigMap ..."
helm template sep ./charts \
  -f ./charts/values.yaml \
  -f "./charts/${ENV}/values.yaml" \
  --show-only templates/configmap-quotas.yaml \
  -n "${NAMESPACE}" | kubectl apply -n "${NAMESPACE}" -f -

echo "============================================================"
echo "🎉 配額更新完成！目前的 sep-system-quotas 內容："
echo "============================================================"
kubectl get configmap sep-system-quotas -n "${NAMESPACE}" -o jsonpath='{.data}' | tr ',' '\n'
echo
