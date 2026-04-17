.PHONY: up down logs ps restart rebuild clean test e2e seed

up:
	docker compose up --build -d
	@echo ""
	@echo "Stack starting. Tail logs with 'make logs', seed demo data with 'make seed'."
	@echo "Frontend: http://localhost:3000   HA: http://localhost:8123   API: http://localhost:8080"

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

# Seed demo users + classroom + sample devices. Idempotent.
seed:
	python3 scripts/seed.py

# End-to-end test — stack must be running (make up). Requires Python 3 + websockets.
e2e:
	python3 scripts/e2e.py
