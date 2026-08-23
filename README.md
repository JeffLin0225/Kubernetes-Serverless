事前準備：
kubectl config current-context 查看目前 kube 叢集

啟動系統：
go run .

健康測試：
 curl http:/localhost:8080/health


kube job 物件測試：
新增任務：
 curl -s -X POST http://localhost:8080/api/run \
  -H "Content-Type: application/json" \
  -d '{
    "system_id": "crawler",
    "task_id": "test-003",
    "image": "prefect-demo:latest",
    "env": {}
  }'

查看 configMap
  kubectl get configmap

套用 configMap 
kubectl apply -f configMap/system-quotas.yaml