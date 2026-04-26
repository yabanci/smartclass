.PHONY: up down logs ps restart rebuild clean test e2e seed verify wait \
        mobile-test mobile-analyze mobile-build

up:
	docker compose up --build -d
	@echo ""
	@echo "Stack starting. Tail logs with 'make logs', seed demo data with 'make seed'."
	@echo "API: http://localhost:8080   HA: http://localhost:8123   MQTT: localhost:1883"

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

# Wait for backend to report healthy + HA self-check to pass. Idempotent —
# polls every 5s for up to 5 minutes. Use this after `make up` so you don't
# have to click around the HA welcome wizard or guess when the stack is
# usable. Exits 0 when the integration stack is green, non-zero otherwise.
wait:
	@python3 scripts/wait_ready.py

# One-shot readiness report: prints the live self-check result from the
# backend and exits non-zero if anything is red. Safe to run repeatedly.
verify:
	@python3 scripts/verify.py

# ── Flutter mobile ─────────────────────────────────────────────────────────

mobile-test:
	cd mobile && flutter test --reporter expanded

mobile-analyze:
	cd mobile && flutter analyze --fatal-infos

mobile-build:
	cd mobile && flutter build apk --release --no-pub
