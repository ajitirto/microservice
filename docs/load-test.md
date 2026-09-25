# Laporan Load Testing

Dokumen teknis: metodologi, hasil, dan temuan eksperimen load testing pada
eksperimen **horizontal scaling** (Phase 6). Konteks bisnis ada di
[README](../README.md).

## Metodologi

- **Tool**: [hey](https://github.com/rakyll/hey) (load generator berbasis Go).
- **Endpoint**: `GET /api/posts` (list post) melalui entry point publik.
- **Parameter**: durasi 10 detik, 30 concurrent worker, token login valid.
- **Rate limit**: `GATEWAY_RATE_LIMIT` dinaikkan ke `100000`/menit untuk
  eksperimen, agar limiter bukan faktor pembatas pengukuran.
- **Snapshot infrastruktur** (sebelum & sesudah):
  - koneksi database (`pg_stat_database.numbackends`) per database
  - Redis hit/miss rate
  - kedalaman antrian RabbitMQ
  - Prometheus: request rate & latensi p95

## Skenario

| Run | Topologi | Keterangan |
|---|---|---|
| Baseline | 1 gateway / 1 user / 1 post | HAProxy tunggal |
| Scaled | 2 gateway / 3 user / 5 post | HAProxy round-robin + gRPC round_robin |

## Hasil

| Metrik | Baseline | Scaled | Perubahan |
|---|---|---|---|
| Total requests (10s) | 20.881 | 25.849 | +24% |
| Requests/sec | 2.085 | 2.583 | +24% |
| p50 | 8,2 ms | 9,2 ms | ~sama |
| p75 | 15,6 ms | 12,0 ms | -23% |
| p90 | 23,9 ms | 15,6 ms | -35% |
| p95 | 30,3 ms | 19,5 ms | **-36%** |
| p99 | 48,0 ms | 37,7 ms | **-21%** |
| Error rate | 0% | 0% | - |
| Slowest | 361 ms | 247 ms | -32% |

Snapshot infrastruktur (scaled):

| Komponen | Nilai | Arti |
|---|---|---|
| postgres `post_db` koneksi | 60 | 5 replica × pool max 12 → **di batas pool** |
| Redis hit rate | 407.696 hits / 107 miss (99,97%) | cache sangat efektif |
| RabbitMQ queue depth | 0 | async tidak menumpuk |
| Prometheus p95 | 15,6 ms | latensi terukur |
| CPU semua container | < 1% | bukan bottleneck CPU |

## Kesimpulan

1. **Scaling horizontal bekerja**: +24% throughput, p95/p99 turun drastis,
   error tetap 0%.
2. **Cache (Redis) bukan bottleneck**: hit rate 99,97%; hampir semua request
   list post dilayani tanpa menyentuh database.
3. **Queue (RabbitMQ) bukan bottleneck**: depth 0; event async tidak menumpuk.
4. **Konstrain pertama adalah koneksi pool Postgres**: 5 replica post × 12
   koneksi = 60 backend tepat di atas kapasitas pool per replica. Langkah
   tuning selanjutnya: perbesar `MaxConns` per replica, atau tambah replica,
   atau tuning Postgres.

## Catatan Metodologi (Gotcha)

- **HAProxy + DNS embedded**: satu baris `server gateway gateway:8080` tidak
  round-robin ke beberapa replica (Docker DNS hanya mengembalikan satu alamat).
  Harus dua `server` eksplisit — lihat `deploy/docker/haproxy/haproxy.scale.cfg`.
- **`docker compose up` tanpa `--scale` me-reconcile replica turun ke 1**: saat
  memperbaiki konfigurasi di tengah eksperimen, replica yang di-scale ikut
  terhapus. Selalu sertakan flag `--scale` pada command berikutnya.
- **Env tidak seragam antar replica**: satu gateway dengan
  `GATEWAY_RATE_LIMIT=60` dan satu dengan `100000` menghasilkan 429 pada
  separuh traffic (masing-masing replica memakai Redis key yang sama tetapi
  limit berbeda). Verifikasi env tiap replica sebelum mengukur.
- **429 ≠ bottleneck service**: status 429 berasal dari rate limiter (Redis),
  bukan saturasi CPU/database. Identifikasi sumber status code sebelum menilai
  bottleneck.

Data mentah: `/tmp/opencode/lt-baseline.log`, `/tmp/opencode/lt-scaled2.log`
(di mesin eksperimen).

## Referensi

- [Arsitektur teknis — Scaling Experiment](architecture.md)
- [Kubernetes](kubernetes.md)