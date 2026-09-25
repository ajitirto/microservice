#!/usr/bin/env bash
set -euo pipefail

URL="${URL:-http://localhost:8080}"
RPS="${RPS:-100}"
DURATION="${DURATION:-10s}"
CONCURRENCY="${CONCURRENCY:-30}"
EMAIL="${EMAIL:-aji@example.com}"
PASS="${PASS:-password123}"
PROMETHEUS="${PROMETHEUS:-http://localhost:9095}"

token="$(curl -s -X POST "$URL/api/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\"}" | jq -r '.access_token')"
echo "token: ${token:0:12}..."

snapshot() {
  local label="$1"
  echo "=== $label ==="
  echo "--- gateway instances ---"
  docker ps --filter "name=microservice-gateway" --format '{{.Names}}'
  echo "--- container cpu/mem ---"
  docker stats --no-stream --format 'table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}' "microservice-gateway-1" "microservice-gateway2-1" "microservice-user-1" "microservice-user-2" "microservice-user-3" "microservice-post-1" "microservice-post-2" "microservice-post-3" "microservice-post-4" "microservice-post-5" "microservice-notification-1" "microservice-postgres" "microservice-redis" 2>/dev/null || true
  echo "--- postgres connections ---"
  docker exec microservice-postgres psql -U postgres -tAc "SELECT datname, numbackends, xact_commit FROM pg_stat_database WHERE datname NOT LIKE 'template%' AND datname NOT LIKE 'postgres'" || true
  echo "--- redis hit rate ---"
  docker exec microservice-redis redis-cli INFO stats | grep -E "keyspace_hits|keyspace_misses" || true
  echo "--- rabbitmq queue depth ---"
  curl -s -u guest:guest http://localhost:15672/api/queues | jq -c '[.[] | {name, messages}]' || true
  echo "--- prometheus http req rate + p95 (5m) ---"
  curl -s "$PROMETHEUS/api/v1/query" --data-urlencode 'query=sum(rate(http_requests_total[5m]))' | jq -c '.data.result[0].value[1]' || true
  curl -s "$PROMETHEUS/api/v1/query" --data-urlencode 'query=histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))' | jq -c '.data.result[0].value[1]' || true
}

run() {
  local label="$1"
  snapshot "before-$label"
  echo "--- hey: ${RPS} rps, ${DURATION}, conc ${CONCURRENCY} ---"
  hey -z "$DURATION" -q "$RPS" -c "$CONCURRENCY" -m GET -H "Authorization: Bearer $token" "$URL/api/posts"
  snapshot "after-$label"
}

run "${1:-run}"