SHELL := /bin/bash
-include .env
export GRAFANA_PORT
MAGE := $(shell command -v mage || echo $(shell go env GOPATH)/bin/mage)

ARCH := $(shell uname -m)
ifeq ($(ARCH),arm64)
  BACKEND_TARGET := build:linuxARM64
else ifeq ($(ARCH),aarch64)
  BACKEND_TARGET := build:linuxARM64
else
  BACKEND_TARGET := build:linux
endif

.PHONY: dev build build-frontend build-backend up bootstrap down clean logs test test-integration e2e dashboard

## dev: build the plugin, start n8n, bootstrap owner/keys/seed workflows, start Grafana
dev: build
	docker compose up -d n8n
	./scripts/dev/bootstrap.sh
	docker compose up -d --build grafana
	@echo "Grafana: http://localhost:$${GRAFANA_PORT:-3000}  n8n: http://localhost:5678"

build: build-frontend build-backend

node_modules: package-lock.json
	npm ci
	@touch node_modules

build-frontend: node_modules
	npm run build

build-backend:
	@test -x $(MAGE) || go install github.com/magefile/mage@latest
	$(MAGE) -v $(BACKEND_TARGET)

## bootstrap: owner, API keys and seed workflows only
bootstrap:
	./scripts/dev/bootstrap.sh

## down: stop the stack, keep data
down:
	docker compose down

## clean: stop the stack and delete n8n/Postgres data
clean:
	docker compose down -v

logs:
	docker compose logs -f grafana n8n

test:
	go test ./pkg/...
	npm run typecheck
	npm run lint
	npm run test:ci

## test-integration: queries against the instances configured in .env
test-integration:
	set -a; . ./.env; set +a; go test -tags integration ./pkg/plugin -run Integration -v -count=1

## e2e: Playwright tests against the running dev stack
e2e:
	GRAFANA_URL=http://localhost:$${GRAFANA_PORT:-3000} npm run e2e

## dashboard: regenerate the bundled dashboard from scripts/dev/gen-dashboard.py
dashboard:
	python3 scripts/dev/gen-dashboard.py
	npx prettier --write src/dashboards
