# Database — PostgreSQL

Dokumen teknis: storage persisten yang digunakan oleh seluruh service.
Konteks bisnis dan gambaran umum ada di [README](../README.md).

## Prinsip

Mengikuti **database ownership** (lihat Architecture §8): setiap service memiliki
database sendiri dan tidak pernah melakukan query langsung ke database service lain.

| Service       | Database          |
|---------------|-------------------|
| auth          | `auth_db`         |
| user          | `user_db`         |
| post          | `post_db`         |
| notification  | `notification_db` |

## Arsitektur Storage

Satu instance PostgreSQL (`postgres:16-alpine`) melayani empat database.
Setiap service terhubung hanya ke database miliknya sendiri lewat `DATABASE_URL`.

```text
postgres:16-alpine
├── auth_db
├── user_db
├── post_db
└── notification_db
```

- **Driver**: `pgx/v5` + connection pool (`pgxpool`).
- **Skema**: setiap service meng-embed `schema.sql` via `go:embed` dan
  menerapkannya saat startup (idempotent) sekaligus melakukan seed.
- **Fallback**: jika `DATABASE_URL` kosong, service otomatis memakai repository
  in-memory (mode awal POC) — memudahkan eksperimen tanpa database.

## Pembuatan Database

File `deploy/docker/postgres/init.sql` membuat empat database di atas,
dieksekusi sekali oleh entrypoint Postgres ketika volume data masih kosong.

- Docker Compose: di-mount ke `/docker-entrypoint-initdb.d/init.sql`.
- Kubernetes: menjadi ConfigMap `postgres-init` yang di-mount ke path yang sama
  (lihat [kubernetes.md](kubernetes.md)).

## Skema per Service

### auth — `auth_db`

- `credentials` — id, email, password_hash
- `refresh_tokens` — token refresh untuk sesi

### user — `user_db`

- `users` — id, name, email, dan data profil

### post — `post_db`

- `posts` — id, title, content, author_id, timestamp
- `post_likes` — relasi suka; jumlah like dihitung dengan `COUNT`

### notification — `notification_db`

- `notifications` — id, routing_key, payload (JSONB)

## Seed (data awal)

| Service | Seed |
|---|---|
| auth | `aji@example.com` (password di-hash saat runtime) |
| user | user `123` — "Aji" |
| post | post `p1`, `p2` |
| notification | notifikasi contoh `n1` |

Proses seed berjalan otomatis saat startup service (idempotent).

## Keterbatasan yang Diketahui

- **Idempotency tetap in-memory**: key idempotency pada post service belum
  dipindahkan ke tabel database (mis. `idempotency_keys`), sehingga key hilang
  saat post service restart. Area perbaikan jika ingin dipersistenkan.
- **init.sql hanya berjalan sekali**: database hanya dibuat saat volume baru;
  penghapusan volume data diperlukan jika ingin menjalankan ulang init.

## Verifikasi

```bash
# Docker Compose
docker compose exec postgres psql -U postgres -c '\l'

# Kubernetes
kubectl exec -n poc deploy/postgres -- psql -U postgres -c '\l'
```

Daftar database harus menampilkan `auth_db`, `user_db`, `post_db`, `notification_db`.

## Referensi

- [Arsitektur teknis (Database Ownership)](architecture.md)
- [Kubernetes (deployment & wiring)](kubernetes.md)