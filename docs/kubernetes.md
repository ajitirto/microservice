# Kubernetes (kind, lokal)

Dokumen teknis: deployment microservices ke Kubernetes di cluster **kind** lokal.
Konteks bisnis dan gambaran umum ada di [README](../README.md).

## Lingkup

Fase 7 — Production Infrastructure, dengan batasan project:

- Cluster: **kind** yang sudah ada (single node, port 80/443 terpetakan ke host).
- **Tidak menggunakan container registry** — image dimuat langsung ke node
  dengan `kind load docker-image`.
- **Tidak ada cloud deployment** — seluruhnya berjalan di mesin lokal.
- CI (GitHub Actions) terpisah dan hanya mengecek kualitas kode, lihat [ci-cd.md](ci-cd.md).

## Topologi

```text
host localhost:80/443
        │ ingress-nginx (kind)
        ▼
    Ingress gateway (/)
        │
        ▼
   Service: gateway:8080
        │
        ├── http → auth:8081 / user:8082 / post:8083
        ├── gRPC → auth:9091 / user:9090
        └── redis / rabbitmq / postgres (dependency)
```

- Namespace: `poc`
- **Service discovery** memakai DNS Kubernetes: service saling memanggil lewat
  nama Service (mis. `http://auth:8081`, `user:9090`) — pengganti Docker DNS
  pada fase docker-compose.

## Isi `deploy/k8s/` (kustomize)

| File | Isi |
|---|---|
| `namespace.yaml` | namespace `poc` |
| `configmap.yaml` | konfigurasi non-sensitif (URL service, timeout, rate limit) |
| `secret.yaml` | kredensial: password Postgres, `AUTH_SECRET`, `DATABASE_URL` per service |
| `postgres.yaml` | Deployment + Service + PVC `postgres-data` (1Gi), init.sql via ConfigMap |
| `redis.yaml` | Deployment + Service |
| `rabbitmq.yaml` | Deployment + Service (AMQP 5672, management 15672) |
| `auth.yaml` / `user.yaml` / `post.yaml` / `notification.yaml` | Deployment + Service per service |
| `gateway.yaml` | Deployment + Service (8080) |
| `ingress.yaml` | Ingress nginx → `gateway:8080` |
| `prometheus.yaml` | Deployment + Service + ConfigMap scrape config (5 target) |
| `kustomization.yaml` | daftar resources |

```bash
kubectl apply -k deploy/k8s
```

## Proses Deploy

Semua otomatis di `deploy/scripts/k8s-up.sh`:

1. Pastikan `ingress-nginx` (manifest khusus kind) terpasang.
   `kubectl label node kind-control-plane ingress-ready=true`.
2. Build image: `microservice-{gateway,auth,user,post,notification}:latest`.
3. `kind load docker-image` untuk image service + `postgres:16-alpine`,
   `redis:7-alpine`, `rabbitmq:3-management-alpine`, `prom/prometheus:latest`.
4. `kubectl apply -k deploy/k8s` (ConfigMap `postgres-init` dibuat dari
   `deploy/docker/postgres/init.sql` sebelum apply).
5. Tunggu rollout semua deployment; jalankan smoke test via `http://localhost`.

Tear-down: `deploy/scripts/k8s-down.sh` (hapus namespace `poc`, termasuk PVC).

## Gotcha yang Ditemukan (penting)

### 1. Env port ter-inject Kubernetes menimpa konfigurasi service

Kubernetes otomatis menyuntikkan env `AUTH_PORT`, `USER_PORT`, `POST_PORT`, dll
ke **semua pod** dalam satu namespace dengan format `tcp://<cluster-ip>:<port>`.
Service membaca `getEnv("AUTH_PORT", "8081")` → mendapat `tcp://10.96.x.x:8081`
→ crash dengan `listen tcp: address :tcp://...: too many colons`.

Solusi: definisikan env eksplisit di pod spec (nilai eksplisit mengalahkan
env inject K8s):

```yaml
env:
  - name: AUTH_PORT
    value: "8081"
```

### 2. ConfigMap `postgres-init` harus berada di namespace `poc`

`kubectl create configmap postgres-init` tanpa `-n poc` membuatnya di `default`,
dan pod postgres gagal mount (`configmap "postgres-init" not found`).

### 3. Probe RabbitMQ di single-node

Probe liveness awal (initialDelay 20s, period 15s) terlalu agresif untuk
single-node kind; RabbitMQ sempat start lalu di-kill berulang (exit 0 karena
penutupan graceful saat SIGTERM). Solusi: proba dilonggarkan
(initialDelay 45s, timeout 10s, failureThreshold 6) + memory limit 1Gi.

## Observability

`prometheus.yaml` menjalankan Prometheus yang men-scrape `/metrics` kelima
service (interval 5s). Akses:

```bash
kubectl port-forward -n poc svc/prometheus 9090
```

## Smoke Test (hasil verifikasi)

Berjalan otomatis di `k8s-up.sh`, hasil end-to-end terverifikasi:

- `GET /health` → `{"status":"ok"}`
- login `aji@example.com` → token
- create post → 201 dengan id baru
- list posts → daftar konten beserta nama penulis (gRPC ke user)
- like → event async `post_liked`
- consumer notification mencatat `event processed` (`post_created`, `post_liked`)
  dengan `trace_id` yang melintasi gateway → post → broker → notification
- Prometheus: 5/5 target `up`

## Referensi

- [Arsitektur teknis](architecture.md)
- [Database (PostgreSQL)](database.md)
- [CI (GitHub Actions)](ci-cd.md)
- [Laporan load testing](load-test.md)