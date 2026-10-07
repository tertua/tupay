.PHONY: clean critic security lint test build run webui.check dev-be dev-fe db.backup db.restore promote promote-prod check-flow check.imports

APP_NAME = tupay
BUILD_DIR = $(PWD)/build
VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")
LDFLAGS := -w -s -X github.com/tertua/tupay/pkg/constants.Version=$(VERSION)

clean:
	rm -rf ./build

# Packages are listed explicitly so third-party code under webui/node_modules
# is never linted. hugeParam/rangeValCopy are style/perf suggestions whose
# refactors would churn many call sites for no behavior change.
critic:
	gocritic check -enableAll -disable hugeParam,rangeValCopy ./app/... ./pkg/... ./platform/... ./docs/... .

security:
	gosec ./...

lint:
	golangci-lint run ./...

test: clean critic security lint check.imports
	go test -v -timeout 30s -coverprofile=cover.out -cover ./...
	go tool cover -func=cover.out

# Controllers must route through platform/gateway (registry + capabilities),
# never import a provider package. Required gate (see ARCHITECTURE.md #4).
check.imports:
	node scripts/check-no-provider-imports.mjs

build: test
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) .

run: swag build
	$(BUILD_DIR)/$(APP_NAME)

# Local dev processes (see AGENTS.md "Local dev run"). Binaries and logs
# live under /tmp/opencode (pre-approved scratch dir, never the repo root).
dev-be:
	go build -ldflags="$(LDFLAGS)" -o /tmp/opencode/tupay . && /tmp/opencode/tupay

dev-fe:
	bun run --cwd=webui dev -- --host 0.0.0.0

webui.check:
	bun run --cwd=webui lint
	bun run --cwd=webui test
	bun run --cwd=webui check:charts
	bun run --cwd=webui check:fixtures
	bun run --cwd=webui build
	bun run --cwd=webui check:bundles:strict

# Snapshots land in data/backups/ — SQLite (SQL_DSN empty) or PostgreSQL via
# pg_dump/pg_restore (needs postgresql-client). Restore needs the backend
# stopped and CONFIRM=yes when run non-interactively (see README).
db.backup:
	node scripts/db-snapshot.mjs backup

db.restore:
	node scripts/db-snapshot.mjs restore $(FILE)

docker.run: docker.network docker.postgres swag docker.fiber docker.redis

docker.network:
	docker network inspect template-network >/dev/null 2>&1 || \
	docker network create -d bridge template-network

docker.fiber.build:
	docker build -t tupay .

docker.fiber: docker.fiber.build
	docker run --rm -d \
		--name template-fiber \
		--network template-network \
		-p 5000:5000 \
		tupay

docker.postgres:
	docker run --rm -d \
		--name template-postgres \
		--network template-network \
		-e POSTGRES_USER=postgres \
		-e POSTGRES_PASSWORD=password \
		-e POSTGRES_DB=postgres \
		-v ${HOME}/dev-postgres/data/:/var/lib/postgresql/data \
		-p 5432:5432 \
		postgres

docker.redis:
	docker run --rm -d \
		--name template-redis \
		--network template-network \
		-p 6379:6379 \
		redis

docker.stop: docker.stop.fiber docker.stop.postgres docker.stop.redis

docker.stop.fiber:
	docker stop template-fiber

docker.stop.postgres:
	docker stop template-postgres

docker.stop.redis:
	docker stop template-redis

swag:
	swag init

# Branching: dev daily, main stable, master production.
# Promote only via fast-forward, never merge-commit or force-push.
# Release: make promote && make promote-prod
promote:
	git fetch origin --prune
	git checkout main
	git merge --ff-only origin/dev
	git push origin main
	git checkout dev

# main -> master (ff-only by construction) + annotated tag v$(VERSION), idempotent.
# The tag is created when missing and then always pushed: the push is a no-op
# when origin already has it, but it repairs a tag that only ever existed
# locally. Checking local refs alone was the old bug — after one failed tag
# push (v1.1.0) every later run printed "exists, skip" and origin never got it.
promote-prod:
	git fetch origin --prune
	git push origin origin/main:refs/heads/master
	@git rev-parse -q --verify "refs/tags/v$(VERSION)" >/dev/null || \
		git tag -a "v$(VERSION)" -m "release v$(VERSION)" origin/master
	git push origin "v$(VERSION)"
	@echo "OK: master at $$(git rev-parse --short origin/master), tag v$(VERSION)."

check-flow:
	git fetch origin --prune
	@echo "master..main count (behind ahead): $$(git rev-list --left-right --count origin/master...origin/main)"
	@echo "main..dev count (behind ahead): $$(git rev-list --left-right --count origin/main...origin/dev)"
	@git merge-base --is-ancestor origin/master origin/main || (echo "FAIL: master is not ancestor of main (promote via make promote-prod, ff-only)"; exit 1)
	@git merge-base --is-ancestor origin/main origin/dev || (echo "FAIL: main is not ancestor of dev (needs rebase/ff, no merge-commit/force-push)"; exit 1)
	@test -z "$$(git log --merges --format=%H origin/master..origin/dev)" || (echo "FAIL: merge commits found in master..dev:"; git log --merges --oneline origin/master..origin/dev; exit 1)
	@echo "OK: master ancestor of main ancestor of dev, no merge commits."
