FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates \
    && addgroup -S app \
    && adduser -S -G app app
COPY --from=builder /out/gateway /usr/local/bin/gateway
USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/gateway"]