.PHONY: up down logs ps restart rebuild clean test e2e

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

# End-to-end test — stack must be running (make up). Requires Python 3 + websockets.
e2e:
	python3 scripts/e2e.py
