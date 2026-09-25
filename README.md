# POC Microservices — Belajar Membangun Sistem Terdistribusi

Project ini adalah **laboratorium** untuk mempelajari cara kerja sistem
terdistribusi. Secara produk, yang dibangun adalah **platform konten sederhana**
— pengguna bisa masuk, membuat postingan, melihat postingan, dan mendapat
pemberitahuan — tetapi aplikasinya sengaja dipecah menjadi beberapa layanan
yang berdiri sendiri agar perilaku sistem terdistribusi bisa dipelajari.

Fokus utama **bukan** membuat aplikasi yang siap produksi, melainkan memahami
**bagaimana request bergerak di dalam sistem**, **di mana bottleneck muncul
saat traffic meningkat**, dan **trade-off** yang harus dibayar ketika sebuah
aplikasi dipecah menjadi banyak layanan.

---

## Arsitektur (Tampilan Bisnis)

```text
Pengguna / Client
        │
        ▼
Pintu Depan (API Gateway)
        │
        ├───────────────┬────────────────┐
        ▼               ▼                ▼
   Akun (Auth)     Profil (User)    Konten (Post)
                                         │
                                         ▼ (di belakang layar)
                                   Pemberitahuan (Notification)
```

- **Pintu Depan (API Gateway)** — satu pintu masuk untuk semua pengguna:
  memverifikasi identitas, membatasi akses, dan mencatat setiap kunjungan.
  Pengguna tidak perlu tahu layanan mana yang melayani permintaannya.
- **Layanan inti** — setiap divisi bisnis (akun, profil, konten) ditangani
  layanan yang berdiri sendiri.
- **Pekerjaan di belakang layar** — pemberitahuan dikirim lewat antrian
  sehingga pengguna tidak perlu menunggu proses itu selesai.

---

## Layanan & Peran Bisnis

| Layanan | Peran Bisnis | Contoh Interaksi |
|---|---|---|
| **API Gateway** | Pintu masuk tunggal; mengatur identitas, keamanan, dan catatan lalu lintas | Mengarahkan setiap permintaan ke divisi yang tepat |
| **Auth** | Identitas & sesi pengguna | Masuk, keluar, mengeluarkan token akses |
| **User** | Profil pengguna | Melihat / memperbarui profil |
| **Post** | Konten | Membuat, melihat daftar, dan menyukai postingan |
| **Notification** | Pemberitahuan yang tidak perlu ditunggu | "Postingan kamu berhasil dibuat" |

---

## Alur Bisnis

1. Pengguna masuk melalui pintu depan → diberikan **token** sebagai tanda
   identitas.
2. Pengguna membuat postingan → konten tersimpan di divisi konten.
3. Pemberitahuan dikirim **di belakang layar** (lewat antrian) — pengguna tidak
   menunggu proses ini.
4. Pengguna lain melihat daftar postingan dan menyukainya.

---

## Tujuan Project

- Memahami perjalanan sebuah request melintasi banyak layanan.
- Menemukan titik tersendat (bottleneck) saat traffic meningkat — lihat
  [laporan load testing](docs/load-test.md).
- Menguji praktik nyata yang dipakai di industri: REST, gRPC, cache, antrian,
  keandalan, pemantauan, scaling, dan Kubernetes.
- Memberi gambaran nyata kapan arsitektur ini layak dipakai di bisnis — dan
  kapan tidak.

---

## Filosofi Project (Dari Sisi Bisnis)

- **Setiap divisi berdiri sendiri.** Akun, profil, dan konten masing-masing
  punya "tim" sendiri. Satu divisi bisa dikembangkan atau ditingkatkan
  kapasitasnya tanpa mengubah divisi lain.
- **Kerja berat dipindah ke antrian.** Pekerjaan yang tidak harus selesai saat
  pengguna menunggu (misal notifikasi) dikerjakan di belakang layar, sehingga
  pengalaman pengguna tetap cepat.
- **Pemisahan memberi kelincahan, tetapi ada biayanya.** Lebih banyak divisi
  berarti lebih banyak bagian yang harus dipantau, dihitung, dan dijaga.

---

## Jika Arsitektur Ini Dipakai di Bisnis Nyata

**Keuntungan**

- Skala sesuai kebutuhan: divisi yang ramai ditingkatkan tanpa menyentuh
  divisi lain.
- Isolasi kegagalan: satu divisi bermasalah tidak serta-merta melumpuhkan
  layanan lain.
- Tim lebih otonom: setiap divisi bisa rilis dan berkembang dengan ritmenya
  sendiri.

**Konsekuensi**

- Lebih banyak titik yang harus dipantau; menelusuri masalah lintas layanan
  lebih sulit.
- Komunikasi antar-divisi menambah latensi.
- Konsistensi data menuntut disiplin tinggi (satu database per divisi).
- Membutuhkan investasi pemantauan, keamanan, dan operasional yang lebih besar.

Kesimpulan bisnis: arsitektur ini membayar biaya operasional dengan kelincahan
dan ketahanan. Untuk organisasi kecil, aplikasi tunggal (monolith) sering kali
lebih murah dan lebih cepat dihadirkan.

---

## Hasil yang Sudah Dicapai

- Lima divisi layanan + pintu depan berjalan dalam tiga mode: **Docker
  Compose**, **scaling horizontal**, dan **Kubernetes (kind)** di mesin lokal.
- Eksperimen scaling (2× pintu depan, 3× divisi profil, 5× divisi konten):
  throughput naik **+24%**, latensi p95 turun **36%**, tanpa error —
  [laporan lengkap](docs/load-test.md).
- Penyimpanan data dipindahkan dari memori ke **PostgreSQL** (satu database
  per divisi) — [detail teknis](docs/database.md).
- Pipeline **CI otomatis** di GitHub Actions dengan **ringkasan hasil per run**
  — [detail teknis](docs/ci-cd.md).

---

## Menjalankan Project

```bash
# Mode lokal (Docker Compose)
git clone <repo> && cd microservice
cp .env.example .env
docker compose up --build

# Mode Kubernetes (kind) — otomatis build, load image, deploy, dan smoke test
./deploy/scripts/k8s-up.sh
```

Setelah aktif, aplikasi bisa diakses di `http://localhost` (lihat
[contoh request](docs/architecture.md)).

---

## Dokumentasi Teknis

Semua detail implementasi ada di direktori `docs/`:

| Dokumen | Isi |
|---|---|
| [Arsitektur Teknis](docs/architecture.md) | Desain system, komunikasi (REST/gRPC/event), alur request, cara menjalankan |
| [Database](docs/database.md) | PostgreSQL: database per divisi, skema, seed |
| [Kubernetes](docs/kubernetes.md) | Deployment ke kind: ingress, service discovery, kustomize, gotcha |
| [Laporan Load Testing](docs/load-test.md) | Metodologi & hasil eksperimen scaling (baseline vs scaled) |
| [CI — GitHub Actions](docs/ci-cd.md) | Pipeline continuous integration + job summary |