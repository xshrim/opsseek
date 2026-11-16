SHELL := /bin/bash

COMPOSE_FILE := deploy/compose/docker-compose.yml
WEBUI_DIST := backend/webui/dist
IMAGE_REPOSITORY ?= opskeeper
IMAGE_TAG ?= local
DOCKERFILE ?= deploy/Dockerfile
# Use pipe fallback semantics so an unavailable proxy does not block the
# development workflow. A comma only falls back for 404/410 responses.
# A command-line assignment (for example, `make GOPROXY=...`) can still
# override this project default. Environment-only values are intentionally
# ignored so a stale global GOPROXY cannot disable the fallback chain.
GOPROXY = https://goproxy.cn|https://proxy.golang.org|direct
export GOPROXY
ALPINE_MIRROR ?= https://mirrors.aliyun.com/alpine
NPM_REGISTRY ?= https://registry.npmmirror.com
CGO_ENABLED ?= 0
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_PACKAGE := opskeeper/backend/version
GO_LDFLAGS := -s -w -X $(VERSION_PACKAGE).Value=$(VERSION) -X $(VERSION_PACKAGE).Commit=$(COMMIT) -X $(VERSION_PACKAGE).BuildTime=$(BUILD_TIME)

define docker-image
	docker build \
		--build-arg "GOPROXY=$(GOPROXY)" \
		--build-arg ALPINE_MIRROR=$(ALPINE_MIRROR) \
		--build-arg NPM_REGISTRY=$(NPM_REGISTRY) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--target $(1) -t $(IMAGE_REPOSITORY)-$(1):$(IMAGE_TAG) -f $(DOCKERFILE) .
endef

define compose
@export DOCKER_SOCKET_GID="$${DOCKER_SOCKET_GID:-$$(stat -c '%g' /var/run/docker.sock 2>/dev/null || echo 0)}"; \
if docker compose version >/dev/null 2>&1; then \
	docker compose -f $(COMPOSE_FILE) --env-file .env $(1); \
elif command -v docker-compose >/dev/null 2>&1; then \
	docker-compose -f $(COMPOSE_FILE) --env-file .env $(1); \
else \
	echo "Docker Compose is not installed"; \
	exit 1; \
fi
endef

.DEFAULT_GOAL := help

.PHONY: \
	help deps env sample-seed sample-clean \
	run up down clean \
	db-up db-down db-clean db-logs \
	obs-up obs-down obs-clean obs-logs \
	infra-up infra-down infra-clean infra-logs \
	build api-build worker-build scheduler-build frontend-build webui-assets app-build backend-build \
	api-run worker-run scheduler-run frontend-run app-run \
	image api-image worker-image scheduler-image frontend-image app-image \
	api-up api-down api-clean api-logs \
	worker-up worker-down worker-clean worker-logs \
	scheduler-up scheduler-down scheduler-clean scheduler-logs \
	frontend-up frontend-down frontend-clean frontend-logs \
	app-up app-down app-clean app-logs \
	test backend-test frontend-test backend-embedded-test backend-integration-test \
	mcp-build docker-mcp-build host-mcp-build kubernetes-mcp-build \
	mcp-run docker-mcp-run host-mcp-run kubernetes-mcp-run \
	mcp-image docker-mcp-image host-mcp-image kubernetes-mcp-image \
	mcp-test docker-mcp-test host-mcp-test kubernetes-mcp-test \
	backend-lint frontend-lint deploy-lint helm-lint lint \
	quality format format-check llm-provider-test

help: ## Show available commands.
	@awk 'BEGIN {FS = ":.*## "; printf "OpsKeeper development commands:\n\n"} /^[a-zA-Z_-]+:.*## / {printf "  %-22s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## Download backend and frontend dependencies.
	cd backend && go mod download
	cd frontend && npm install --registry="$(NPM_REGISTRY)"

env: ## Create and synchronize the local environment files.
	@mkdir -p deploy/compose
	@test -f .env || cp .env.example .env
	cp .env.example deploy/compose/.env.example
	cp .env deploy/compose/.env

sample-seed: env ## Insert local sample teams, projects, users, GLM channel, Docker and Host resources, and role bindings.
	@set -a; source .env; set +a; cd backend && go run ./cmd/sampledata seed

sample-clean: env ## Remove only marked local sample teams, projects, users, GLM channel, Docker and Host resources, and bindings.
	@set -a; source .env; set +a; cd backend && go run ./cmd/sampledata clean

run: env db-up app-run ## Start PostgreSQL and run the embedded app locally.

up: env app-image ## Start PostgreSQL and the embedded app container.
	$(call compose,up -d --wait --wait-timeout 180 postgres app)

down: env ## Stop PostgreSQL and the embedded app container.
	$(call compose,stop postgres app)

clean: env ## Remove all local Compose containers, networks, and data volumes.
	$(call compose,down --volumes --remove-orphans)

db-up: env ## Start the PostgreSQL development dependency.
	$(call compose,up -d --wait --wait-timeout 60 postgres)

db-down: env ## Stop the PostgreSQL development dependency.
	$(call compose,stop postgres)

db-clean: env ## Remove the PostgreSQL container and data volume.
	$(call compose,down --volumes postgres)

db-logs: env ## Follow PostgreSQL logs.
	$(call compose,logs -f postgres)

obs-up: env ## Start Jaeger, OpenTelemetry, Prometheus, Loki, and Grafana.
	$(call compose,--profile obs up -d --wait --wait-timeout 60 jaeger otel-collector prometheus loki grafana)

obs-down: env ## Stop the local observability stack.
	$(call compose,--profile obs stop jaeger otel-collector prometheus loki grafana)

obs-clean: env ## Remove observability containers and data volumes.
	$(call compose,--profile obs down --volumes jaeger otel-collector prometheus loki grafana)

obs-logs: env ## Follow local observability stack logs.
	$(call compose,--profile obs logs -f jaeger otel-collector prometheus loki grafana)

infra-up: env db-up obs-up ## Start the database and observability containers.

infra-down: env db-down obs-down ## Stop the database and observability containers.

infra-clean: env db-clean obs-clean ## Remove the database and observability containers and data.

infra-logs: env ## Follow database and observability logs.
	$(call compose,--profile obs logs -f postgres jaeger otel-collector prometheus loki grafana)

# Service artifacts
build: api-build worker-build scheduler-build frontend-build app-build ## Build API, Worker, Scheduler, Frontend, and App artifacts.

api-build: ## Build the API binary.
	mkdir -p backend/bin
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -o bin/opskeeper-api ./cmd/api

worker-build: ## Build the Worker binary.
	mkdir -p backend/bin
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -o bin/opskeeper-worker ./cmd/worker

scheduler-build: ## Build the Scheduler binary.
	mkdir -p backend/bin
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -o bin/opskeeper-scheduler ./cmd/scheduler

frontend-build: ## Build the frontend static assets.
	cd frontend && npm run build

webui-assets: frontend-build ## Copy frontend assets for the embedded App.
	mkdir -p $(WEBUI_DIST)
	find $(WEBUI_DIST) -mindepth 1 -maxdepth 1 -exec rm -rf {} +
	cp -R frontend/dist/. $(WEBUI_DIST)/

app-build: webui-assets ## Build the embedded App binary.
	mkdir -p backend/bin
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -tags=embed_webui -o bin/opskeeper-app ./cmd/api

backend-build: api-build worker-build scheduler-build app-build ## Build all backend service binaries.

# Service processes
api-run: env ## Run the API server directly in the development environment.
	set -a; source .env; set +a; cd backend && go run ./cmd/api

worker-run: env ## Run the Worker directly in the development environment.
	set -a; source .env; set +a; cd backend && go run ./cmd/worker

scheduler-run: env ## Run the Scheduler directly in the development environment.
	set -a; source .env; set +a; cd backend && go run ./cmd/scheduler

frontend-run: env ## Run the frontend development server directly.
	set -a; source .env; set +a; cd frontend && npm run dev

app-run: env webui-assets ## Build and embed the frontend, then run the app directly.
	set -a; source .env; set +a; cd backend && go run -tags=embed_webui ./cmd/api

# Service images and containers
# Host-built image artifacts must be static Linux binaries; leave normal builds native.
ifeq ($(DOCKERFILE),deploy/Dockerfile)
api-image worker-image scheduler-image app-image docker-mcp-image host-mcp-image kubernetes-mcp-image: export GOOS := linux
api-image worker-image scheduler-image app-image docker-mcp-image host-mcp-image kubernetes-mcp-image: CGO_ENABLED := 0
api-image: api-build
worker-image: worker-build
scheduler-image: scheduler-build
frontend-image: frontend-build
app-image: app-build
docker-mcp-image: docker-mcp-build
host-mcp-image: host-mcp-build
kubernetes-mcp-image: kubernetes-mcp-build
endif

image: api-image worker-image scheduler-image frontend-image app-image ## Build all service images.

api-image: ## Build the API image.
	$(call docker-image,api)

worker-image: ## Build the Worker image.
	$(call docker-image,worker)

scheduler-image: ## Build the Scheduler image.
	$(call docker-image,scheduler)

frontend-image: ## Build the standalone frontend image.
	$(call docker-image,frontend)

app-image: ## Build the embedded app image.
	$(call docker-image,app)

api-up: env api-image ## Start the API container.
	$(call compose,--profile svc up -d --wait --wait-timeout 180 api)

api-down: env ## Stop the API container.
	$(call compose,--profile svc stop api)

api-clean: env ## Remove the API container without removing shared data or images.
	$(call compose,--profile svc rm -s -f api)

api-logs: env ## Follow API logs.
	$(call compose,--profile svc logs -f api)

worker-up: env worker-image ## Start the Worker container.
	$(call compose,--profile svc up -d --wait --wait-timeout 180 worker)

worker-down: env ## Stop the Worker container.
	$(call compose,--profile svc stop worker)

worker-clean: env ## Remove the Worker container without removing shared data or images.
	$(call compose,--profile svc rm -s -f worker)

worker-logs: env ## Follow Worker logs.
	$(call compose,--profile svc logs -f worker)

scheduler-up: env scheduler-image ## Start the Scheduler container.
	$(call compose,--profile svc up -d --wait --wait-timeout 180 scheduler)

scheduler-down: env ## Stop the Scheduler container.
	$(call compose,--profile svc stop scheduler)

scheduler-clean: env ## Remove the Scheduler container without removing shared data or images.
	$(call compose,--profile svc rm -s -f scheduler)

scheduler-logs: env ## Follow Scheduler logs.
	$(call compose,--profile svc logs -f scheduler)

frontend-up: env frontend-image ## Start the standalone frontend container.
	$(call compose,--profile svc up -d --wait --wait-timeout 180 frontend)

frontend-down: env ## Stop the standalone frontend container.
	$(call compose,--profile svc stop frontend)

frontend-clean: env ## Remove the frontend container without removing shared data or images.
	$(call compose,--profile svc rm -s -f frontend)

frontend-logs: env ## Follow frontend logs.
	$(call compose,--profile svc logs -f frontend)

app-up: env app-image ## Start the embedded app container.
	$(call compose,up -d --wait --wait-timeout 180 app)

app-down: env ## Stop the embedded app container.
	$(call compose,stop app)

app-clean: env ## Remove the App container without removing shared data or images.
	$(call compose,rm -s -f app)

app-logs: env ## Follow App logs.
	$(call compose,logs -f app)

# Service tests
test: backend-test frontend-test ## Run all unit tests.

backend-test: ## Run backend unit tests.
	cd backend && go test ./...

frontend-test: ## Run frontend unit tests.
	cd frontend && npm run test

backend-embedded-test: webui-assets ## Test embedded Web UI and HTTP API.
	cd backend && go test -tags=embed_webui ./webui ./httpapi

backend-integration-test: ## Run PostgreSQL-backed integration tests.
	set -a; source .env; set +a; cd backend && go test -tags=integration ./migrations ./organization ./identity ./authorization ./resource ./discovery ./connector ./diagnosis ./inspection ./operation ./e2e

# MCP artifacts
mcp-build: docker-mcp-build host-mcp-build kubernetes-mcp-build ## Build all MCP binaries.

docker-mcp-build: ## Build the standalone Docker MCP server binary.
	mkdir -p backend/bin
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -o bin/opskeeper-docker-mcp ./cmd/docker-mcp

host-mcp-build: ## Build the standalone Host MCP agent binary.
	mkdir -p backend/bin
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -o bin/opskeeper-host-mcp-agent ./cmd/host-mcp-agent

kubernetes-mcp-build: ## Build the standalone Kubernetes MCP server binary.
	mkdir -p backend/bin
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -o bin/opskeeper-kubernetes-mcp ./cmd/kubernetes-mcp

# MCP processes
mcp-run: docker-mcp-run host-mcp-run kubernetes-mcp-run ## Run the MCP services.

docker-mcp-run: ## Run the standalone Docker MCP server.
	set -a; source .env; set +a; cd backend && go run ./cmd/docker-mcp

host-mcp-run: ## Run the standalone Host MCP agent.
	set -a; source .env; set +a; cd backend && go run ./cmd/host-mcp-agent

kubernetes-mcp-run: ## Run the standalone Kubernetes MCP server.
	set -a; source .env; set +a; cd backend && go run ./cmd/kubernetes-mcp

# MCP images
mcp-image: docker-mcp-image host-mcp-image kubernetes-mcp-image ## Build all MCP images.

docker-mcp-image: ## Build the Docker MCP image.
	$(call docker-image,docker-mcp)

host-mcp-image: ## Build the Host MCP image.
	$(call docker-image,host-mcp)

kubernetes-mcp-image: ## Build the Kubernetes MCP image.
	$(call docker-image,kubernetes-mcp)

# MCP tests
mcp-test: docker-mcp-test host-mcp-test kubernetes-mcp-test ## Run all MCP tests.

docker-mcp-test: ## Run docker-mcp unit and Docker Engine simulator tests.
	cd backend && go test ./mcpserver/docker/... ./cmd/docker-mcp

host-mcp-test: ## Run Host MCP unit tests.
	cd backend && go test ./tool/host ./mcpserver/host/... ./cmd/host-mcp-agent

kubernetes-mcp-test: ## Run Kubernetes MCP unit tests.
	cd backend && go test ./mcpserver/kubernetes/... ./cmd/kubernetes-mcp

# Lint
backend-lint: ## Run backend static checks.
	cd backend && go vet ./...

frontend-lint: ## Run frontend static checks.
	cd frontend && npm run check

deploy-lint: ## Validate deployment shell scripts.
	sh -n deploy/compose/postgres/check-ready.sh deploy/compose/postgres/init/001-create-opskeeper.sh

helm-lint: ## Validate and render the production Helm chart.
	helm lint deploy/helm/opskeeper
	helm template opskeeper deploy/helm/opskeeper >/dev/null

lint: backend-lint frontend-lint deploy-lint helm-lint ## Run all static checks.

# Other commands
quality: format-check lint test backend-embedded-test build ## Run the complete local quality gate.

format: ## Format source files.
	cd backend && gofmt -w .
	cd frontend && npm run format

format-check: ## Check source formatting without modifying files.
	@files=$$(cd backend && gofmt -l .); test -z "$$files" || (echo "Unformatted Go files:"; echo "$$files"; exit 1)
	cd frontend && npm run format:check

llm-provider-test: ## Test the configured external LLM through the ADK Runner.
	set -a; source .env; set +a; cd backend && go test -tags=integration ./llm -run TestSiliconFlowThroughADKRunner -v
