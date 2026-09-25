#!/usr/bin/env bash
set -euo pipefail

NS="poc"
CONTEXT="kind-kind"

echo "==> context: ${CONTEXT}"
kubectl config use-context "${CONTEXT}" >/dev/null

echo "==> hapus resource namespace ${NS}"
kubectl delete namespace "${NS}" --wait=true 2>/dev/null || true

echo "==> selesai. namespace ${NS} dihapus (postgres PVC ikut terhapus)."
echo "    ingress-nginx dibiarkan terpasang agar k8s-up.sh berikutnya lebih cepat."