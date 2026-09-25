## POC-Microservice 

### struktur file 

```bash
microservice-poc/
├── gateway/
│   └── ...
├── services/
│   ├── auth/
│   │   └── ...
│   ├── user/
│   │   └── ...
│   ├── post/
│   │   └── ...
│   └── notification/
│       └── ...
├── deploy/
│   └── docker/
├── docs/
│   └── architecture.md
├── docker-compose.yml
├── .env.example
├── Makefile
├── README.md
└── .gitignore
```

```bash

Client
  │
  │ REST
  ▼
API Gateway
  │
  ├── REST → Auth Service
  ├── REST → User Service
  └── REST → Post Service
                  │
                  │ gRPC
                  ▼
             User Service

```




Proof of Concept untuk mempelajari arsitektur **microservices**, komunikasi antar-service, API Gateway, REST API, gRPC, Docker, caching, dan asynchronous event processing.

Project ini menggunakan **monorepo**, sehingga seluruh service berada dalam satu repository untuk memudahkan development dan eksperimen secara lokal.

## Architecture

```text
                         ┌──────────────┐
                         │    Client    │
                         │ Web / Mobile │
                         └───────┬──────┘
                                 │
                                REST
                                 │
                                 ▼
                        ┌─────────────────┐
                        │   API Gateway   │
                        │      :8080      │
                        └────────┬────────┘
                                 │
                ┌────────────────┼────────────────┐
                │                │                │
               REST             REST             REST
                │                │                │
                ▼                ▼                ▼
        ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
        │ Auth Service │ │ User Service │ │ Post Service │
        │    :8001     │ │    :8002     │ │    :8003     │
        └──────────────┘ └──────┬───────┘ └──────┬───────┘
                                │                 │
                                │      gRPC       │
                                └────────┬────────┘
                                         │
                                         ▼
                                ┌──────────────────┐
                                │ Other Services   │
                                └──────────────────┘

                         Post Service
                              │
                              │ Event
                              ▼
                     ┌────────────────────┐
                     │ Notification       │
                     └────────────────────┘
                     │ Service            │
```

## Goals

Project ini dibuat sebagai laboratory untuk memahami:

* API Gateway
* Microservice architecture
* REST API
* gRPC
* Service-to-service communication
* Authentication
* Authorization
* Database per service
* Redis caching
* Asynchronous processing
* Event-driven architecture
* Docker networking
* Horizontal scaling
* Health check
* Timeout dan retry
* Observability
* CI/CD

Fokus utama project bukan membuat aplikasi production-ready, tetapi memahami **bagaimana sistem terdistribusi bekerja dan di mana bottleneck muncul ketika traffic meningkat**.

---

## Services

### API Gateway

Entry point untuk client.

Responsibilities:

* Routing
* Authentication middleware
* Rate limiting
* Request ID
* Logging
* Timeout
* Proxy request
* Response handling

Port:

```text
8080
```

Contoh:

```http
GET /api/users/123
```

Gateway meneruskan request ke:

```text
user-service:8002
```

---

### Auth Service

Mengelola authentication dan identity.

Responsibilities:

* Register
* Login
* Logout
* Access token
* Refresh token
* Credential validation
* Token verification

Port:

```text
8001
```

Contoh:

```http
POST /auth/login
POST /auth/refresh
POST /auth/logout
```

---

### User Service

Mengelola data dan domain user.

Responsibilities:

* User profile
* User settings
* User preferences
* User information

Port:

```text
8002
```

Contoh:

```http
GET /users/me
GET /users/:id
PUT /users/:id
```

---

### Post Service

Mengelola post/article.

Responsibilities:

* Create post
* Get post
* Update post
* Delete post
* List posts
* Like post

Port:

```text
8003
```

Contoh:

```http
GET    /posts
GET    /posts/:id
POST   /posts
PUT    /posts/:id
DELETE /posts/:id
POST   /posts/:id/like
```

---

### Notification Service

Mengelola proses notification secara asynchronous.

Contoh event:

```text
PostCreated
    │
    ▼
Message Broker
    │
    ▼
Notification Service
    │
    ▼
Email / Notification
```

Service ini digunakan untuk mempelajari asynchronous processing sehingga Post Service tidak perlu menunggu notification selesai.

---

# Communication

## REST

REST digunakan terutama untuk komunikasi yang berhubungan dengan client.

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

REST juga dapat digunakan untuk beberapa komunikasi antar-service yang sederhana.

---

## gRPC

gRPC digunakan untuk komunikasi internal antar-service ketika membutuhkan contract yang jelas dan komunikasi request-response yang efisien.

Contoh:

```text
Post Service
     │
     │ gRPC
     ▼
User Service
```

Misalnya Post Service membutuhkan informasi user:

```text
GetUser(user_id)
```

dengan protobuf sebagai contract.

---

## Event

Untuk proses asynchronous:

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

Dengan pendekatan ini Post Service tidak harus menunggu Notification Service menyelesaikan pekerjaannya.

---

# Repository Structure

```text
.
├── gateway/
│   ├── cmd/
│   ├── internal/
│   ├── Dockerfile
│   ├── go.mod
│   └── go.sum
│
├── services/
│   ├── auth/
│   │   ├── cmd/
│   │   ├── internal/
│   │   ├── proto/
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   └── go.sum
│   │
│   ├── user/
│   │   ├── cmd/
│   │   ├── internal/
│   │   ├── proto/
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   └── go.sum
│   │
│   ├── post/
│   │   ├── cmd/
│   │   ├── internal/
│   │   ├── proto/
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   └── go.sum
│   │
│   └── notification/
│       ├── cmd/
│       ├── internal/
│       ├── Dockerfile
│       ├── go.mod
│       └── go.sum
│
├── proto/
│   ├── user.proto
│   ├── post.proto
│   └── auth.proto
│
├── deploy/
│   └── docker/
│
├── docs/
│   └── architecture.md
│
├── docker-compose.yml
├── Makefile
├── .env.example
├── .gitignore
└── README.md
```

## Service Structure

Setiap service dibuat relatif independen.

Contoh:

```text
services/user/

├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── handler/
│   ├── service/
│   ├── repository/
│   ├── model/
│   ├── grpc/
│   └── config/
│
├── migrations/
├── proto/
├── Dockerfile
├── go.mod
└── go.sum
```

Dependency flow:

```text
Handler
   │
   ▼
Service
   │
   ▼
Repository
   │
   ▼
Database
```

Business logic tidak ditempatkan di API Gateway.

---

# Docker

Semua service dijalankan menggunakan Docker Compose.

Contoh network:

```text
microservice-network
        │
        ├── gateway
        ├── auth-service
        ├── user-service
        ├── post-service
        ├── notification-service
        ├── postgres
        ├── redis
        └── message-broker
```

Service dapat berkomunikasi menggunakan nama container/service.

Contoh:

```text
http://user-service:8002
```

Bukan:

```text
http://localhost:8002
```

karena `localhost` di dalam container mengarah ke container itu sendiri.

---

# Example Request

Client mengakses:

```http
GET http://localhost:8080/api/users/123
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
  ├── Rate limit
  ├── Add request ID
  └── Route request
          │
          ▼
    User Service
          │
          ▼
       Database
```

Response:

```json
{
  "id": 123,
  "name": "Aji",
  "email": "aji@example.com"
}
```

---

# Example gRPC Flow

Post Service membutuhkan data user.

```text
Post Service
      │
      │ GetUser(123)
      ▼
 User Service
      │
      ▼
 User Database
```

Contract didefinisikan menggunakan protobuf:

```proto
service UserService {
  rpc GetUser(GetUserRequest) returns (UserResponse);
}
```

Dengan demikian service tidak perlu mengetahui implementasi internal User Service.

---

# Database Ownership

Setiap service memiliki ownership terhadap datanya sendiri.

```text
Auth Service
    │
    └── Auth Database

User Service
    │
    └── User Database

Post Service
    │
    └── Post Database
```

Service lain **tidak melakukan direct query** ke database service lain.

Contoh yang dihindari:

```text
Post Service
     │
     └── SELECT * FROM user_db.users
```

Sebagai gantinya:

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

Ini menjaga ownership dan boundary antar-service.

---

# Environment

Copy environment example:

```bash
cp .env.example .env
```

Contoh konfigurasi:

```env
GATEWAY_PORT=8080

AUTH_SERVICE_URL=http://auth-service:8001
USER_SERVICE_URL=http://user-service:8002
POST_SERVICE_URL=http://post-service:8003

REDIS_URL=redis://redis:6379

DATABASE_HOST=postgres
DATABASE_PORT=5432
```

---

# Running

Build dan jalankan seluruh stack:

```bash
docker compose up --build
```

Background:

```bash
docker compose up -d --build
```

Melihat container:

```bash
docker compose ps
```

Melihat log:

```bash
docker compose logs -f
```

Log service tertentu:

```bash
docker compose logs -f gateway
docker compose logs -f user-service
docker compose logs -f post-service
```

Stop:

```bash
docker compose down
```

Stop sekaligus menghapus volume:

```bash
docker compose down -v
```

---

# Health Check

Setiap service menyediakan endpoint health check:

```http
GET /health
```

Contoh:

```bash
curl http://localhost:8080/health
```

Expected:

```json
{
  "status": "ok"
}
```

---

# Scaling Experiment

Salah satu tujuan POC ini adalah melakukan eksperimen horizontal scaling.

Contoh menjalankan beberapa instance:

```bash
docker compose up --scale user-service=3
```

Kemudian:

```text
             API Gateway
                  │
        ┌─────────┼─────────┐
        ▼         ▼         ▼
     User #1   User #2   User #3
```

Eksperimen dapat dilakukan untuk mengamati:

* CPU
* Memory
* Request per second
* Latency
* Database connection
* Redis hit rate
* Network latency
* Error rate

---

# Learning Roadmap

## Phase 1 — Basic Microservices

* [ ] API Gateway
* [ ] Auth Service
* [ ] User Service
* [ ] Post Service
* [ ] Docker Compose
* [ ] Internal Docker network

## Phase 2 — Communication

* [ ] REST API
* [ ] gRPC
* [ ] Protobuf
* [ ] Service-to-service communication
* [ ] Timeout
* [ ] Retry

## Phase 3 — Performance

* [ ] Redis
* [ ] Caching
* [ ] Connection pooling
* [ ] Database indexing
* [ ] Rate limiting
* [ ] Load testing

## Phase 4 — Async Architecture

* [ ] Message broker
* [ ] Event
* [ ] Queue
* [ ] Consumer
* [ ] Retry
* [ ] Dead letter queue

## Phase 5 — Reliability

* [ ] Health check
* [ ] Circuit breaker
* [ ] Graceful shutdown
* [ ] Idempotency
* [ ] Distributed tracing
* [ ] Metrics
* [ ] Structured logging

## Phase 6 — Scaling

* [ ] Horizontal scaling
* [ ] Load balancing
* [ ] Multiple gateway instances
* [ ] Multiple service instances
* [ ] Database bottleneck
* [ ] Cache bottleneck
* [ ] Queue bottleneck

## Phase 7 — Production Infrastructure

* [x] Kubernetes (kind lokal)
* [x] Service discovery (Kubernetes DNS)
* [x] Ingress (ingress-nginx)
* [x] Observability stack (Prometheus)
* [x] CI/CD (GitHub Actions — Continuous Integration + job summary)
* [ ] Container registry (khusus lokal via `kind load`, tidak push registry)
* [ ] Cloud deployment (khusus lokal, tidak deploy ke cloud)

---

# Design Principles

### 1. Gateway bukan business logic

Gateway bertanggung jawab terhadap:

```text
Routing
Authentication
Rate Limiting
Timeout
Observability
```

Business logic berada di service.

### 2. Service memiliki domain

```text
Auth Service       → Authentication
User Service       → User
Post Service       → Post
Notification       → Notification
```

### 3. Database ownership

Service tidak melakukan direct query ke database service lain.

### 4. REST untuk external API

REST/JSON digunakan sebagai interface yang mudah dikonsumsi client.

### 5. gRPC untuk internal communication

gRPC digunakan ketika service membutuhkan komunikasi internal dengan contract yang kuat.

### 6. Event untuk asynchronous processing

Proses yang tidak harus selesai dalam request utama dapat dipindahkan ke event/queue.

---

# Project Philosophy

Project ini sengaja dibuat bertahap.

Tidak semua teknologi langsung digunakan sejak awal.

Urutan pembelajaran:

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

Tujuannya bukan sekadar memiliki banyak container, tetapi memahami **trade-off dan bottleneck sistem terdistribusi**.
