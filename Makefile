GO ?= go
PYTHON ?= python3
SERVICES := routershelf routershelf-smb
HOST_BINARIES := $(addprefix dist/,$(SERVICES))
MIPS_BINARIES := $(addsuffix -linux-mipsle,$(HOST_BINARIES))

.PHONY: test check check-shell check-config check-format check-docs vet test-race test-repository \
        build build-mipsle build-smb-mipsle build-router nat-plan test-stress-tool

test: check-shell check-config test-stress-tool test-repository
	$(GO) test ./...

# CI and the local full check use the same commands. No deployment is performed.
check: test check-format check-docs vet test-race

check-shell:
	@for script in deploy/padavan/*.sh scripts/*.sh examples/status-dashboard/scripts/*.sh examples/status-dashboard/config.example.sh; do \
		sh -n "$$script" || exit; \
	done

check-format:
	@unformatted=$$(gofmt -l cmd internal) && { test -z "$$unformatted" || { echo "Run go fmt ./...: $$unformatted"; exit 1; }; }

check-docs:
	$(PYTHON) -B scripts/check_repository.py

vet:
	$(GO) vet ./...

test-race:
	$(GO) test -race ./...

# Native executables are useful for local development and macOS client checks.
build: $(HOST_BINARIES)

# Preserve the existing per-service targets and artifact names.
build-mipsle: dist/routershelf-linux-mipsle
build-smb-mipsle: dist/routershelf-smb-linux-mipsle
build-router: $(MIPS_BINARIES)

# Always rebuild: a package depends on embedded assets and transitive sources.
.PHONY: $(HOST_BINARIES) $(MIPS_BINARIES)
$(HOST_BINARIES):
	@mkdir -p dist
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags='-s -w' -o $@ ./cmd/$(notdir $@)

$(MIPS_BINARIES): dist/%-linux-mipsle:
	@mkdir -p dist
	GOOS=linux GOARCH=mipsle GOMIPS=softfloat CGO_ENABLED=0 $(GO) build -trimpath -ldflags='-s -w' -o $@ ./cmd/$*

check-config:
	ROUTERSHELF_ENV_FILE="$(CURDIR)/.env.example" sh deploy/padavan/supervisor.example.sh --check-config

nat-plan:
	sh scripts/nat-plan.sh

test-stress-tool:
	$(PYTHON) -B -m unittest discover -s scripts/tests -p 'test_stress_test.py' -v

test-repository:
	$(PYTHON) -B -m unittest discover -s scripts/tests -p 'test_repository.py' -v
