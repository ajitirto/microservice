#!/usr/bin/env bash
set -euo pipefail

NS="poc"
CONTEXT="kind-kind"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

echo "==> context: ${CONTEXT}"
kubectl config use-context "${CONTEXT}" >/dev/null

echo "==> pastikan ingress-nginx (kind) terpasang"
if ! kubectl get ns ingress-nginx >/dev/null 2>&1; then
  kubectl label node kind-control-plane ingress-ready=true --overwrite
  kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.13.1/deploy/static/provider/kind/deploy.yaml
  kubectl rollout status deployment/ingress-nginx-controller -n ingress-nginx --timeout=180s >/dev/null
else
  kubectl label node kind-control-plane ingress-ready=true --overwrite
fi

echo "==> build image service"
for img in gateway auth user post notification; do
  ctx="services/${img}"
  [ "${img}" = "gateway" ] && ctx="gateway"
  docker build -q -t "microservice-${img}:latest" -f "${ctx}/Dockerfile" "${ctx}" >/dev/null
  echo "    built microservice-${img}:latest"
done

echo "==> kind load docker-image"
for img in microservice-gateway:latest microservice-auth:latest microservice-user:latest \
           microservice-post:latest microservice-notification:latest \
           postgres:16-alpine redis:7-alpine rabbitmq:3-management-alpine prom/prometheus:latest; do
  kind load docker-image "${img}" >/dev/null
  echo "    loaded ${img}"
done

echo "==> apply kustomize (${ROOT}/deploy/k8s)"
kubectl create configmap postgres-init --from-file="${ROOT}/deploy/docker/postgres/init.sql" \
  --namespace "${NS}" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl apply -k "${ROOT}/deploy/k8s"

echo "==> tunggu semua deployment ready"
for d in postgres redis rabbitmq auth user post notification gateway prometheus; do
  kubectl rollout status "deployment/${d}" -n "${NS}" --timeout=180s >/dev/null
  echo "    ${d}: ready"
done

echo "==> smoke test via ingress (http://localhost)"
TOKEN="$(curl -s -X POST http://localhost/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"aji@example.com","password":"password123"}' | jq -r '.access_token')"
echo "    login: ok"
curl -s -X POST http://localhost/api/posts \
  -H "Authorization: Bearer ${TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"title":"k8s-up smoke","content":"deployed via kind"}' >/dev/null
echo "    create post: ok"
curl -s http://localhost/api/posts -H "Authorization: Bearer ${TOKEN}" >/dev/null
echo "    list posts: ok"

echo "==> selesai. akses: http://localhost (ingress) | prometheus: kubectl port-forward -n ${NS} svc/prometheus 9090"