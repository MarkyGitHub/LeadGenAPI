.PHONY: help build up down logs clean test

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build Docker images
	docker-compose build

up: ## Start all services
	docker-compose up -d

down: ## Stop all services
	docker-compose down

logs: ## View logs from all services
	docker-compose logs -f

logs-api: ## View API server logs
	docker-compose logs -f api

logs-worker: ## View worker logs
	docker-compose logs -f worker

ps: ## Show running containers
	docker-compose ps

restart: ## Restart all services
	docker-compose restart

restart-api: ## Restart API server
	docker-compose restart api

restart-worker: ## Restart worker
	docker-compose restart worker

clean: ## Stop services and remove volumes
	docker-compose down -v

test-webhook: ## Send a test webhook
	curl -X POST http://localhost:8080/webhooks/leads \
		-H "Content-Type: application/json" \
		-d '{"email":"test@example.com","phone":"+49123456789","zipcode":"66123","house":{"is_owner":true}}'

health: ## Check API health
	curl http://localhost:8080/health

stats: ## Get lead statistics
	curl http://localhost:8080/stats/leads/counts

recent: ## Get recent leads
	curl http://localhost:8080/stats/leads/recent

shell-api: ## Open shell in API container
	docker-compose exec api sh

shell-worker: ## Open shell in worker container
	docker-compose exec worker sh

shell-db: ## Open PostgreSQL shell
	docker-compose exec postgres psql -U postgres -d lead_gateway

backup-db: ## Backup database
	docker-compose exec postgres pg_dump -U postgres lead_gateway > backup_$$(date +%Y%m%d_%H%M%S).sql

test: ## Run Go tests
	go test -v ./...

test-coverage: ## Run tests with coverage
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

lint: ## Run linter
	golangci-lint run

dev: ## Run in development mode with live reload
	docker-compose up --build
