.PHONY: up down logs ps restart rebuild clean test

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

ps:
	docker compose ps

restart:
	docker compose restart

rebuild:
	docker compose build --no-cache
	docker compose up -d

# Full reset: removes all containers + volumes (Postgres data, HA config, MQTT state)
clean:
	docker compose down -v

test:
	cd backend && go test ./... -count=1
