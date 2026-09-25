# CI — GitHub Actions (Continuous Integration)

Dokumen teknis: pipeline Continuous Integration project.
Konteks bisnis dan gambaran umum ada di [README](../README.md).

## Lingkup

Sesuai keputusan project, pipeline berhenti di **Continuous Integration**:

- ✅ Check format (`gofmt`)
- ✅ `go vet`
- ✅ `go build`
- ✅ `go test -race` + coverage
- ✅ **Job summary** pada setiap run
- ❌ Tidak ada push container registry
- ❌ Tidak ada deployment (cloud/lokal) otomatis

Deployment ke kind tetap manual via `deploy/scripts/k8s-up.sh`
(lihat [kubernetes.md](kubernetes.md)).

## Workflow

File: `.github/workflows/ci.yml`

### Trigger

```yaml
on:
  push:
    branches: [main, master]
  pull_request:
  workflow_dispatch:
```

`workflow_dispatch` memungkinkan menjalankan CI manual (mis. sebelum pull
request untuk melihat summary).

### Job `ci` (matrix)

Satu job per service (`gateway`, `auth`, `user`, `post`, `notification`),
berjalan paralel dengan `fail-fast: false` sehingga semua hasil tetap tercatat.

```yaml
strategy:
  fail-fast: false
  matrix:
    service: [gateway, auth, user, post, notification]
```

Setiap job menjalankan 4 langkah dan menulis hasil ke `$GITHUB_OUTPUT`:

| Langkah | Output |
|---|---|
| Check formatting (`gofmt -l .`) | `gofmt=PASS/FAIL` |
| `go vet ./...` | `vet=PASS/FAIL` |
| `go build ./...` | `build=PASS/FAIL` |
| `go test -race -v -coverprofile` | `test=PASS/FAIL`, `tests=<jumlah>`, `coverage=<x.x%>`, `failures=<log>` |

Langkah test tidak pernah mem-fail job sendiri (nilai `exit 0`); status
diserahkan ke job summary agar **satu run menghasilkan satu laporan terpadu**.

### Job `summary` (agregator)

```yaml
summary:
  needs: ci
  if: always()
```

Menjalankan `toJSON(needs.ci.outputs)` lalu menulis tabel markdown ke
`$GITHUB_STEP_SUMMARY`:

```text
## CI Summary

| Service | gofmt | vet | build | test (race) | tests | coverage |
|---|---|---|---|---|---|---|
| gateway | ✅ | ✅ | ✅ | ✅ | 51 | 71,4% |
| auth    | ✅ | ✅ | ✅ | ✅ | 46 | ... |
| ...     |     |     |     |     |     |     |
```

- Verdict keseluruhan (✅ semua hijau / ❌ ada yang gagal) + info run
  (event, branch, commit, actor).
- Tetap tampil walau ada job gagal (`if: always()`), dan menandai run
  dengan status gagal bila ada satu saja check yang FAIL.

## Cara Melihat Summary

1. Buka tab **Actions** → pilih salah satu run.
2. Gulir ke bawah halaman run → section **Summary** milik job `CI Summary`.

Hasil langkah per service tetap bisa dilihat di job `ci (<service>)`
masing-masing (termasuk log kegagalan lengkap).

## Referensi

- [Arsitektur teknis](architecture.md)
- [Kubernetes (deployment lokal)](kubernetes.md)