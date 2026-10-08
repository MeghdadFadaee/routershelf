GO ?= go
PYTHON ?= python3
BINARY := dist/routershelf-linux-mipsle

.PHONY: test build-mipsle check-shell check-config nat-plan test-stress-tool

test: check-shell check-config test-stress-tool
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

test-stress-tool:
	$(PYTHON) -B -m unittest discover -s scripts/tests -p 'test_*.py' -v
