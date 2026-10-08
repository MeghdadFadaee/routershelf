GO ?= go
BINARY := dist/routershelf-linux-mipsle

.PHONY: test build-mipsle check-shell check-config nat-plan

test: check-shell check-config
	$(GO) test ./...

check-shell:
	sh -n deploy/padavan/supervisor.example.sh
	sh -n scripts/nat-plan.sh
	sh -n examples/status-dashboard/scripts/status-service.sh
	sh -n examples/status-dashboard/scripts/status-response.sh

build-mipsle:
	mkdir -p dist
	GOOS=linux GOARCH=mipsle GOMIPS=softfloat CGO_ENABLED=0 $(GO) build -trimpath -ldflags='-s -w' -o $(BINARY) ./cmd/routershelf

check-config:
	ROUTERSHELF_ENV_FILE="$(CURDIR)/.env.example" sh deploy/padavan/supervisor.example.sh --check-config

nat-plan:
	sh scripts/nat-plan.sh
