## POC-Microservice 

Microservice adalah sebuah gaya arsitektur yang membangun 1 aplikasi  terdiri dari dua atau lebih service. 

(sumber)[https://microservices.io/]

## Alur Kerja Sistem (Data Flow)

**Order Service** menerima request, memvalidasi pesanan, dan menerbitkan (Publish) pesan ke RabbitMQ.

**Product Service** mengelola inventaris dan menggunakan Redis untuk mempercepat pembacaan data produk.

**Payment Service** mendengarkan (Subscribe) antrean pesan dan memproses pembayaran secara asinkron.

## Technical Stack

Language: Golang 1.21+ (Standard Library & Gin Framework)

Architecture: Clean Architecture (Entities, Use Cases, Repositories) - DDD Inspired.

Diagram: Mermaid (Diagram as a code )

Message Broker: RabbitMQ (Asynchronous Processing)

Database: PostgreSQL (Main Data Store) & Redis (Caching)

Security: JWT Authentication & OWASP Prevention (Input Validation)

DevOps: Docker & Docker Compose for Local Developmen


## Docker Compose

Infrastruktur dijalankan dalam satu perintah menggunakan Docker Compose:

PostgreSQL (Port 5432) - Penyimpanan data utama.

Redis (Port 6379) - Layer caching untuk Product Service.

RabbitMQ (Port 5672 & Management UI 15672) - Jalur komunikasi antar-service.

Order Service (Port 8080)

Product Service (Port 8081)

Payment Service (Background Worker)

## Implementasi Keamanan & Performa

API Security: Menggunakan Middleware untuk validasi JWT pada endpoint sensitif.

Database: Implementasi Connection Pooling dan Transaction untuk integritas data.

Caching: Data produk yang sering diakses disimpan di Redis untuk mengurangi beban database.

Scalability: Layanan Notification dapat di-scale menjadi beberapa kontainer untuk menangani antrean pesan yang besar (Load Balancing).

## project Stucture


```bash
.
├── services/
│   ├── order-service/        #  👈 Producer: Create orders
│   ├── product-service/      #  👈 PInventory & Redis Caching
│   └── payment-service/      #  👈 PConsumer: Process payments
├── common/                   #  👈 PShared logic (JWT, Wrappers)
├── docs/
├── docker-compose.yaml
└── README.md

```

**ket** 

Shared Library: common/ atau pkg/ untuk kode yang dipakai bersama (seperti utilitas JWT atau logger) agar tidak terjadi duplikasi kode.

Environment Variables: .env.example di setiap service.

API Documentation:  api.http
