.PHONY: up down build restart logs ps clean test

up:
	docker compose up -d

down:
	docker compose down

build:
	docker compose build

restart:
	docker compose down
	docker compose up -d

logs:
	docker compose logs -f

ps:
	docker compose ps

test:
	go test ./...

clean:
	docker compose down -v --remove-orphans

rebuild:
	docker compose down
	docker compose build --no-cache
	docker compose up -d
