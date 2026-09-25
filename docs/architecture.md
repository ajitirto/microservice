
# Architecture

Dokumen ini menjelaskan arsitektur Microservice POC, komunikasi antar-service, ownership data, dan alur request.

Dokumentasi teknikal lain:
- [Database (PostgreSQL)](database.md)
- [Kubernetes (kind)](kubernetes.md)
- [CI — GitHub Actions](ci-cd.md)
- [Laporan load testing](load-test.md)

## 1. High-Level Architecture

```text
                              ┌──────────────┐
                              │    Client    │
                              │ Web / Mobile │
                              └───────┬──────┘
                                      │
                                     REST
                                      │
                                      ▼
                           ┌─────────────────────┐
                           │    API Gateway      │
                           │       :8080         │
                           └──────────┬──────────┘
                                      │
                    ┌─────────────────┼─────────────────┐
                    │                 │                 │
                   REST              REST              REST
                    │                 │                 │
                    ▼                 ▼                 ▼
             ┌────────────┐    ┌────────────┐    ┌────────────┐
             │    Auth    │    │    User    │    │    Post    │
             │  Service   │    │  Service   │    │  Service   │
             │   :8001    │    │   :8002    │    │   :8003    │
             └─────┬──────┘    └─────┬──────┘    └─────┬──────┘
                   │                  │                 │
                   ▼                  ▼                 ▼
              Auth DB             User DB           Post DB

                                      ▲
                                      │
                                     gRPC
                                      │
                              ┌───────┴────────┐
                              │ Internal       │
                              │ Communication  │
                              └────────────────┘

                                      │
                                Event / Queue
                                      │
                                      ▼
                            ┌──────────────────┐
                            │   Notification   │
                            │     Service      │
                            └──────────────────┘
```

## 2. Architecture Principles

Arsitektur ini menggunakan beberapa prinsip utama:

1. Client tidak berkomunikasi langsung dengan internal service.
2. API Gateway menjadi single entry point.
3. Setiap service memiliki domain yang jelas.
4. Setiap service memiliki ownership terhadap datanya.
5. Service tidak melakukan direct query ke database service lain.
6. REST digunakan terutama untuk external API.
7. gRPC digunakan untuk komunikasi internal.
8. Event/queue digunakan untuk proses asynchronous.
9. Service harus dapat di-scale secara independen.

---

# 3. API Gateway

API Gateway merupakan entry point dari external client.

```text
Client
  │
  │ HTTP
  ▼
API Gateway
```

Client tidak perlu mengetahui lokasi internal service.

Contoh client request:

```http
GET /api/users/123
Authorization: Bearer <token>
```

Gateway menentukan request tersebut harus diteruskan ke:

```text
http://user-service:8002/users/123
```

## Gateway Responsibilities

Gateway dapat menangani:

* Request routing
* Authentication middleware
* Rate limiting
* Request ID
* Logging
* Timeout
* Retry
* CORS
* Metrics
* Tracing

Gateway **tidak bertanggung jawab terhadap business logic domain**.

Contoh yang tidak dilakukan Gateway:

```text
Gateway
  ├── UserRepository
  ├── UserModel
  ├── UserBusinessLogic
  └── UserDatabase
```

Business logic tetap berada pada User Service.

---

# 4. Service Boundary

Setiap service memiliki domain masing-masing.

```text
Auth Service
    │
    └── Authentication / Identity

User Service
    │
    └── User Profile / User Domain

Post Service
    │
    └── Post / Article Domain

Notification Service
    │
    └── Notification Domain
```

## Auth Service

Auth Service bertanggung jawab terhadap:

* Login
* Register
* Logout
* Password verification
* Access token
* Refresh token
* Session
* Token validation

Contoh endpoint:

```http
POST /auth/register
POST /auth/login
POST /auth/refresh
POST /auth/logout
```

Auth Service tidak bertanggung jawab terhadap seluruh profile user.

---

## User Service

User Service bertanggung jawab terhadap:

* User profile
* User settings
* User preferences
* User information

Contoh endpoint:

```http
GET /users/me
GET /users/:id
PUT /users/:id
DELETE /users/:id
```

---

## Post Service

Post Service bertanggung jawab terhadap:

* Create post
* Read post
* Update post
* Delete post
* List post
* Like post

Contoh endpoint:

```http
GET /posts
GET /posts/:id
POST /posts
PUT /posts/:id
DELETE /posts/:id
POST /posts/:id/like
```

---

## Notification Service

Notification Service bertanggung jawab terhadap:

* Email notification
* Push notification
* Notification history
* Background notification processing

Notification tidak harus dipanggil synchronously oleh Post Service.

Contoh:

```text
Post Created
     │
     ▼
Event / Queue
     │
     ▼
Notification Service
     │
     ▼
Email / Push
```

---

# 5. REST Communication

REST digunakan sebagai external API.

```text
Client
   │
   │ HTTP/JSON
   ▼
API Gateway
```

Contoh:

```http
GET /api/users/123
```

Gateway kemudian melakukan proxy:

```text
GET /api/users/123
        │
        ▼
GET http://user-service:8002/users/123
```

Response diteruskan kembali ke client.

```text
User Service
      │
      ▼
API Gateway
      │
      ▼
Client
```

REST dipilih karena:

* Mudah dikonsumsi client
* Mudah debugging
* Browser friendly
* HTTP standard
* JSON mudah dibaca
* Cocok untuk public API

---

# 6. gRPC Communication

gRPC digunakan untuk komunikasi internal antar-service.

Contoh:

```text
Post Service
      │
      │ gRPC
      ▼
User Service
```

Misalnya Post Service membutuhkan informasi user.

```text
Post Service
      │
      │ GetUser(user_id)
      ▼
User Service
      │
      ▼
User Database
```

Contract didefinisikan menggunakan Protocol Buffers.

Contoh:

```proto
service UserService {
  rpc GetUser(GetUserRequest) returns (UserResponse);
}
```

## Kenapa gRPC?

gRPC memberikan:

* Strong contract
* Protocol Buffers
* Code generation
* Binary serialization
* HTTP/2
* Streaming support
* Cocok untuk internal service communication

---

# 7. REST vs gRPC

Pemilihan protocol berdasarkan boundary komunikasi.

| Communication        | Protocol       |
| -------------------- | -------------- |
| Web → Gateway        | REST           |
| Mobile → Gateway     | REST           |
| Public API → Gateway | REST           |
| Gateway → Auth       | REST/gRPC      |
| Gateway → User       | REST           |
| Gateway → Post       | REST           |
| User → User internal | gRPC           |
| Post → User internal | gRPC           |
| Service → Service    | gRPC           |
| Event processing     | Message Broker |

Tidak semua komunikasi harus menggunakan gRPC.

Prinsipnya:

```text
External API
     │
     └── REST

Internal synchronous communication
     │
     └── gRPC

Internal asynchronous communication
     │
     └── Event / Queue
```

---

# 8. Database Ownership

Setiap service memiliki database atau schema yang menjadi tanggung jawabnya.

```text
┌──────────────┐
│ Auth Service │
└──────┬───────┘
       │
       ▼
   Auth DB


┌──────────────┐
│ User Service │
└──────┬───────┘
       │
       ▼
   User DB


┌──────────────┐
│ Post Service │
└──────┬───────┘
       │
       ▼
   Post DB
```

Service lain tidak melakukan direct database access.

### Tidak diperbolehkan

```text
Post Service
      │
      │ SELECT users
      ▼
User Database
```

### Yang digunakan

```text
Post Service
      │
      │ gRPC
      ▼
User Service
      │
      ▼
User Database
```

Hal ini menjaga:

* Service boundary
* Data ownership
* Independensi deployment
* Independensi schema
* Evolusi service

---

# 9. Authentication Flow

Authentication menggunakan Auth Service.

```text
Client
  │
  │ POST /api/auth/login
  ▼
API Gateway
  │
  ▼
Auth Service
  │
  ├── Validate credentials
  ├── Verify password
  └── Generate token
  │
  ▼
Client
```

Client kemudian menggunakan token:

```http
GET /api/users/me
Authorization: Bearer <token>
```

Flow:

```text
Client
  │
  ▼
API Gateway
  │
  ├── Validate token
  │
  ▼
User Service
  │
  ▼
User DB
```

Gateway dapat melakukan token verification sebelum request diteruskan.

---

# 10. Request Flow

Contoh request mengambil profile:

```http
GET /api/users/123
```

Flow:

```text
Client
  │
  ▼
API Gateway
  │
  ├── Request ID
  ├── Authentication
  ├── Rate Limit
  └── Routing
  │
  ▼
User Service
  │
  ├── Validate request
  ├── Business logic
  └── Repository
  │
  ▼
User DB
```

Response:

```text
User DB
   │
   ▼
User Service
   │
   ▼
API Gateway
   │
   ▼
Client
```

---

# 11. Post Creation Flow

Post creation menggunakan synchronous request.

```text
Client
  │
  │ POST /api/posts
  ▼
API Gateway
  │
  ▼
Post Service
  │
  ├── Validate user
  │       │
  │       └── gRPC → User Service
  │
  ├── Create Post
  │
  ▼
Post DB
```

Setelah post berhasil dibuat, Post Service dapat menghasilkan event:

```text
Post Service
     │
     │ PostCreated
     ▼
Message Broker
     │
     ▼
Notification Service
```

Notification tidak menghambat response utama.

---

# 12. Event-Driven Communication

Event digunakan untuk pekerjaan yang tidak membutuhkan response langsung.

Contoh:

```text
POST /posts
     │
     ▼
Post Service
     │
     ├── Save Post
     │
     └── Publish PostCreated
              │
              ▼
        Message Broker
              │
              ▼
       Notification Service
```

Dengan pendekatan ini:

```text
Post Service
      │
      └── tidak perlu menunggu
                  │
                  ▼
          Notification Service
```

Hal ini membantu mengurangi coupling antar-service.

---

# 13. Docker Network

Semua container berada pada Docker network yang sama.

```text
microservice-network

├── gateway
├── auth-service
├── user-service
├── post-service
├── notification-service
├── redis
├── postgres
└── message-broker
```

Docker menyediakan DNS internal berdasarkan nama service.

Contoh:

```text
http://user-service:8002
```

Gateway tidak menggunakan:

```text
http://localhost:8002
```

karena `localhost` dari dalam container menunjuk ke container Gateway itu sendiri.

---

# 14. Horizontal Scaling

Setiap service dapat di-scale secara independen.

Contoh:

```text
                 Load Balancer
                       │
             ┌─────────┼─────────┐
             ▼         ▼         ▼
          User #1   User #2   User #3
```

Post Service juga dapat memiliki jumlah instance berbeda:

```text
                 Load Balancer
                       │
             ┌─────────┼─────────┐
             ▼         ▼         ▼
          Post #1    Post #2    Post #3
```

Tujuannya adalah memungkinkan scaling berdasarkan bottleneck masing-masing service.

Contoh:

```text
User Service  → 3 replicas
Post Service  → 5 replicas
Auth Service  → 2 replicas
```

Jumlah replica tidak harus sama.

---

# 15. Resilience

Komunikasi antar-service dapat mengalami kegagalan.

Contoh:

```text
Gateway
   │
   ▼
User Service
   X
 unavailable
```

Gateway harus memiliki timeout.

```text
Request
   │
   ▼
User Service
   │
   └── timeout
        │
        ▼
     504 Gateway Timeout
```

Untuk komunikasi tertentu dapat digunakan:

* Timeout
* Retry
* Circuit breaker
* Backoff
* Health check
* Graceful shutdown

Retry tidak boleh digunakan secara sembarangan, terutama untuk operasi yang tidak idempotent.

---

# 16. Observability

Setiap service menghasilkan telemetry.

```text
Services
   │
   ├── Logs
   ├── Metrics
   └── Traces
          │
          ▼
   Observability Stack
```

Request ID digunakan untuk mengikuti satu request.

Contoh:

```text
Request ID: 7f82a1
```

Flow:

```text
Gateway
  │ request_id=7f82a1
  ▼
Post Service
  │ request_id=7f82a1
  ▼
User Service
  │ request_id=7f82a1
  ▼
Database
```

Hal ini memudahkan debugging distributed system.

---

# 17. Failure Scenario

Misalnya User Service mengalami masalah.

```text
Client
  │
  ▼
Gateway
  │
  ▼
User Service
  X
```

Gateway tidak seharusnya terus menunggu tanpa batas.

```text
User Service
      │
      └── timeout
            │
            ▼
       Gateway
            │
            ▼
    504 Gateway Timeout
```

Jika dependency terus gagal, circuit breaker dapat digunakan:

```text
Normal

Gateway → User Service


Failure

Gateway → User Service
             X

Circuit Open

Gateway ─────────X────────→ User Service
```

---

# 18. Scaling Experiment

POC ini dapat digunakan untuk melakukan eksperimen.

### Baseline

```text
1 Gateway
1 Auth Service
1 User Service
1 Post Service
1 Database
```

Kemudian tingkatkan menjadi:

```text
1 Gateway
3 User Services
5 Post Services
```

Bandingkan:

* Requests per second
* P50 latency
* P95 latency
* P99 latency
* Error rate
* CPU
* Memory
* Database connections
* Redis hit rate

Tujuan eksperimen adalah menemukan bottleneck.

---

# 19. Architecture Evolution

Arsitektur dikembangkan secara bertahap.

```text
Phase 1
Client
  │
  ▼
Gateway
  │
  ▼
Services
```

↓

```text
Phase 2
Gateway
  │
  ├── REST
  └── gRPC
        │
     Services
```

↓

```text
Phase 3
Services
  │
  ├── Redis
  └── Database
```

↓

```text
Phase 4
Services
  │
  └── Message Broker
         │
         ▼
      Consumers
```

↓

```text
Phase 5
       Load Balancer
              │
        ┌─────┴─────┐
        ▼           ▼
    Gateway 1   Gateway 2
        │           │
        └─────┬─────┘
              ▼
         Services
```

↓

```text
Phase 6

              Kubernetes
                   │
       ┌───────────┼───────────┐
       ▼           ▼           ▼
    Gateway      Services    Workers
       │           │           │
       └───────────┼───────────┘
                   │
          Database / Redis
```

---

# 20. Trade-offs

Microservices memberikan independensi antar-service, tetapi juga menambah kompleksitas.

### Keuntungan

* Independent deployment
* Independent scaling
* Clear domain boundary
* Technology flexibility
* Failure isolation
* Team ownership

### Konsekuensi

* Network latency
* Distributed failure
* Debugging lebih sulit
* Deployment lebih kompleks
* Data consistency lebih sulit
* Monitoring lebih penting
* Local development lebih berat

Karena itu arsitektur ini digunakan sebagai **POC pembelajaran**, bukan asumsi bahwa semua aplikasi harus menggunakan microservices.

---

# 21. Target Learning

Setelah POC selesai, kemampuan yang ingin diperoleh:

```text
                    Microservices
                         │
        ┌────────────────┼────────────────┐
        ▼                ▼                ▼
   API Gateway        Service         Communication
        │                │                │
        ▼                ▼                ▼
     Routing          Boundary        REST / gRPC
        │                │                │
        └────────────────┼────────────────┘
                         ▼
                    Reliability
                         │
                         ▼
                     Scaling
                         │
                         ▼
                   Observability
```

Fokus utama bukan jumlah service, tetapi memahami **bagaimana request bergerak melalui distributed system dan bagaimana sistem bereaksi ketika salah satu komponennya mengalami bottleneck atau failure**.

---

# 22. Learning Roadmap

Project dikerjakan bertahap. Urutan pembelajaran:

```text
Monolith
   │
   ▼
Microservices
   │
   ▼
API Gateway
   │
   ▼
REST
   │
   ▼
gRPC
   │
   ▼
Redis
   │
   ▼
Queue / Event
   │
   ▼
Observability
   │
   ▼
Horizontal Scaling
   │
   ▼
Kubernetes
```

Status pengerjaan per fase:

## Phase 1 — Basic Microservices

* [x] API Gateway
* [x] Auth Service
* [x] User Service
* [x] Post Service
* [x] Docker Compose
* [x] Internal Docker network

## Phase 2 — Communication

* [x] REST API
* [x] gRPC
* [x] Protobuf
* [x] Service-to-service communication
* [x] Timeout
* [x] Retry

## Phase 3 — Performance

* [x] Redis
* [x] Caching
* [x] Connection pooling
* [x] Database indexing
* [x] Rate limiting
* [x] Load testing

## Phase 4 — Async Architecture

* [x] Message broker
* [x] Event
* [x] Queue
* [x] Consumer
* [x] Retry
* [x] Dead letter queue

## Phase 5 — Reliability

* [x] Health check
* [x] Circuit breaker
* [x] Graceful shutdown
* [x] Idempotency
* [x] Distributed tracing
* [x] Metrics
* [x] Structured logging

## Phase 6 — Scaling

* [x] Horizontal scaling
* [x] Load balancing
* [x] Multiple gateway instances
* [x] Multiple service instances
* [x] Database bottleneck
* [x] Cache bottleneck
* [x] Queue bottleneck

Hasil dan analisis: [load-test.md](load-test.md).

## Phase 7 — Production Infrastructure

* [x] Kubernetes (kind lokal)
* [x] Service discovery (Kubernetes DNS)
* [x] Ingress (ingress-nginx)
* [x] Observability stack (Prometheus)
* [x] CI/CD (GitHub Actions — CI + job summary)
* [ ] Container registry (khusus lokal via `kind load`, tidak push registry)
* [ ] Cloud deployment (khusus lokal, tidak deploy ke cloud)

Detail: [kubernetes.md](kubernetes.md), [ci-cd.md](ci-cd.md).

---

# 23. Menjalankan Project

## Prasyarat

- Docker + Docker Compose
- Go 1.26.x (untuk pengembangan service)
- kind + kubectl (untuk fase Kubernetes)

## Mode Docker Compose

```bash
cp .env.example .env
docker compose up --build        # foreground
docker compose up -d --build     # background
docker compose ps
docker compose logs -f gateway
docker compose down
docker compose down -v           # hapus volume (data database hilang)
```

## Mode Kubernetes (kind)

```bash
./deploy/scripts/k8s-up.sh       # build + kind load + apply + smoke test
./deploy/scripts/k8s-down.sh     # hapus namespace poc
```

## Environment (Docker Compose)

Variabel utama di `.env.example`:

```env
GATEWAY_PORT=8080
AUTH_SERVICE_URL=http://auth:8081
USER_SERVICE_URL=http://user:8082
POST_SERVICE_URL=http://post:8083
NOTIFICATION_SERVICE_URL=http://notification:8084
AUTH_GRPC_ADDR=auth:9091
USER_GRPC_ADDR=user:9090
REDIS_URL=redis://redis:6379
RABBITMQ_URL=amqp://guest:guest@rabbitmq:5672/
DATABASE_URL=postgres://postgres:postgres@postgres:5432/<db>
GATEWAY_RATE_LIMIT=60
```

---

# 24. Health Check & Contoh Request

## Health Check

Setiap service (dan gateway) menyediakan `GET /health`:

```bash
curl http://localhost:8080/health
```

Response:

```json
{"status": "ok"}
```

## Contoh Request

```http
GET http://localhost:8080/api/users/123
Authorization: Bearer <token>
```

```bash
# login → token
curl -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"aji@example.com","password":"password123"}'

# create post (idempotency key)
curl -X POST http://localhost:8080/api/posts \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: key-123' \
  -d '{"title":"Postingan pertama","content":"..."}'

# list posts
curl http://localhost:8080/api/posts -H "Authorization: Bearer $TOKEN"

# like post
curl -X POST http://localhost:8080/api/posts/p1/like \
  -H "Authorization: Bearer $TOKEN"
```

---

# 25. Design Principles

1. **Gateway bukan business logic** — routing, auth, rate limit, timeout, observability.
2. **Service memiliki domain** — auth (identitas), user (profil), post (konten), notification (pemberitahuan).
3. **Database ownership** — service tidak melakukan query langsung ke database service lain (lihat §8).
4. **REST untuk external API** — interface konsumsi client.
5. **gRPC untuk internal communication** — komunikasi antar-service dengan contract kuat.
6. **Event untuk asynchronous processing** — proses non-kritis dipindah ke queue.

---

# 26. Repository Structure

```text
.
├── gateway/                 # API Gateway (Go)
├── services/
│   ├── auth/
│   ├── user/
│   ├── post/
│   └── notification/
├── proto/                   # protobuf contract
├── deploy/
│   ├── docker/              # compose-adjacent (haproxy, postgres, prometheus)
│   ├── k8s/                 # manifest Kubernetes (kustomize)
│   └── scripts/             # loadtest.sh, k8s-up.sh, k8s-down.sh
├── docs/                    # dokumentasi (repo ini)
├── .github/workflows/       # CI (ci.yml)
├── docker-compose.yml
├── .env.example
└── README.md                # gambaran bisnis + link ke dokumentasi
```
