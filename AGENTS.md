# RouterShelf agent guide

These instructions apply to the entire repository. Read any more specific `AGENTS.md` in a directory before changing files there. Follow the user's current request and existing session authorization when deciding the scope of work.

## Project and entry points

RouterShelf has independent Go services for public read-only HTTP/HTTPS and private authenticated SMB storage on constrained Linux routers. The web service provides NAIS directory listings and static pages from the **same shared root**. The original deployment used Padavan-derived firmware and a MikroTik gateway; this is not universal firmware support.

Read the relevant guide before changing a component:

- `README.md` and `docs/architecture.md`: project overview and responsibilities.
- `docs/configuration.md`: environment configuration, precedence and reload.
- `docs/static-pages.md`: index precedence, inline assets and download behavior.
- `docs/getting-started.md` and `docs/operations.md`: installation, updates, rollback and removal.
- `SECURITY.md`: public access and filesystem boundaries.
- `docs/deployment-history.md`: observed results and outstanding hardware checks.

## Repository layout

- `cmd/routershelf/`: web process startup, TLS, bounded listeners, environment/flag parsing and configuration tests.
- `internal/web/`: public HTTP handlers, listings, static-file policy, redirect and behavior tests.
- `cmd/routershelf-smb/`: independent private SMB executable; `main.go` owns CLI/runtime setup, `config.go` private JSON validation, `service.go` serving/storage monitoring, and `storage.go` mount/privilege operations.
- `internal/smbwire/`: adapted Apache-2.0 protocol implementation. Read `docs/smb.md`, `docs/smb-implementation.md`, `docs/smb-validation.md` and `internal/smbwire/UPSTREAM.md` before changing it; preserve license/provenance.
- `internal/theme/assets.go`: embeds assets under `internal/theme/nais/`.
- `deploy/padavan/`: environment-driven supervisor and host firewall lifecycle.
- `scripts/nat-plan.sh`: prints gateway configuration guidance; it does not modify the gateway.
- `scripts/check_repository.py`: checks Git-visible source paths, local documentation links and service metadata without opening ignored private files.
- `scripts/stress_test.py` and `scripts/tests/`: bounded load runner and localhost-only runner tests. Read `docs/stress-testing.md`; writing/editing the tool does not imply permission to load a live target.
- `.env.example`: tracked configuration template; `.env` is private and ignored.
- `examples/status-dashboard/`: independent historical BusyBox/Netcat example, with its guide in `docs/examples/`. Do not expose it using the public server's NAT rules.
- `docs/`, `.github/`, `Makefile`, `project.json`: documentation, CI, local commands and metadata.

Keep changes in the appropriate layer. The Go server must not provision DHCP, change router NAT or configure FTP accounts. Preserve the upstream NAIS license and document local theme adaptations.

## Configuration contract

Per-installation settings never require source edits. The existing web service keeps its `RS_*` environment contract for domains, IPs, subnets, interfaces, ports, USB paths and runtime options. The independent SMB service uses protected JSON selected by `RS_SMB_CONFIG`, with its own tracked template and configuration reference. Use public placeholders in `.env.example` and documentation. Defaults should be generic and safe; the standalone Go listener defaults to loopback.

The supervisor sources a trusted administrator-owned env file, normally `/etc/storage/routershelf.env`; `ROUTERSHELF_ENV_FILE` overrides its location. It exports values and passes application flags. The Go binary reads exported environment values but does **not** load or execute `.env` itself. CLI flags override environment defaults. Keep these representations consistent.

When adding or changing a setting, update its parsing/validation, `.env.example`, the supervisor or NAT-plan consumer when applicable, and `docs/configuration.md`. Test precedence, derived paths and invalid input. Private configuration must never enter Git, public ZIPs, fixtures or diagnostic output.

Editing env does not reload running processes. Config changes require transferring the private file and restarting the verified supervisor; they do not require rebuilding. Code and embedded-theme changes require a rebuild. Gateway changes are applied separately, guided by the NAT plan.

## Behavior and security contracts

Preserve these unless the user explicitly requests a change:

- A directory with a readable regular `index.html` renders it before the NAIS listing. Without an index, the listing remains available. Directory slash redirects allow relative asset links.
- Direct HTML and recognized web assets are served inline with explicit MIME types. Other files remain attachment downloads. Listing links must not force HTML downloads.
- Static pages may run browser-side JavaScript and load assets; PHP/CGI/server-side code is never executed. Do not apply the listing's restrictive CSP to static pages in a way that breaks the documented behavior.
- Only GET/HEAD are allowed; retain HEAD and byte ranges. Public sharing has no authentication, and HTTPS alone does not make content private.
- Use `os.OpenRoot` confinement. Reject hidden paths, traversal, escaping symlinks and nonregular file responses. Keep names escaped and reserve `/_nais/` for bundled assets.
- Keep certificate keys/cache, CA bundle, binary, env and backups outside the shared root. exFAT mode bits do not isolate these from broadly authorized disk accounts.
- Keep domain whitelisting, outbound TLS validation, privilege dropping, connection bounds and timeouts. Do not treat `GOMEMLIMIT` as a hard memory quota.
- Modify only service-owned firewall rules. Preserve unrelated Internet NAT, SSH and firmware rules. Reconcile changes without opening the service unintentionally, and inspect fail-closed temporary blocks if commands fail.

## Local development and verification

Run from the repository root using Go 1.26 or a newer toolchain verified against the target kernel:

```sh
make test           # shell/config checks, Python tool tests, Go tests
make check          # test plus format, docs/hygiene, vet and race checks
make build          # both native executables under dist/
make build-router   # both static MIPS little-endian soft-float executables
make test-stress-tool # Python localhost-only tests; no live-router requests
make check-shell    # syntax only; does not execute deployment
make check-config   # side-effect-free validation using .env.example
make build-mipsle   # dist/routershelf-linux-mipsle, static Linux/MIPS LE soft-float
```

For a private configuration check, use `ROUTERSHELF_ENV_FILE=/absolute/path/to/.env sh deploy/padavan/supervisor.example.sh --check-config`. The env file is sourced shell code: do not load an untrusted one. `make nat-plan` reads private `.env` and prints deployment addresses; do not run it just to produce public diagnostics.

Python 3.10+ is required for runner tests on the development computer, not the router. There is no frontend build. Format Go with `gofmt`; use `.editorconfig` for other text. Keep deployment scripts compatible with the observed `/bin/sh` and BusyBox features.

Use `testing` and `net/http/httptest` for meaningful behavior checks. For handler changes cover the affected filesystem boundaries, methods, headers, index precedence, inline MIME/disposition, fallback listing and byte ranges. For config changes cover env/flag precedence and validation. For shell-only changes run syntax and dry config checks; never start NTP or alter a live firewall merely to check formatting. Documentation-only edits normally need link/accuracy checks, not a cross-build or deployment.

A successful cross-build is not a kernel-compatibility, reboot, renewal or load test. Record actual checks and unresolved limits. Future certificate renewal and full reboot claims require observed target results.

## Live deployment and repository hygiene

A repository edit does not automatically synchronize the router. Deploy only when requested or authorized by the session. Follow `docs/operations.md`: upload a new filename, preserve a known-good private backup, rename into place and restart the appropriate process. Verify PID command lines before signaling; wait for supervisor cleanup before starting another instance. Retain the certificate cache and shared data.

Installed names such as `public-directory`, `public-share-service.sh` and `/tmp/public-share-*` are compatibility paths. Do not rename them just to match the repository name without a complete migration. USB binary updates do not require a flash save; changed `/etc/storage` env/scripts/hooks do on the observed firmware. Avoid unnecessary flash writes.

Keep diffs focused and update user-facing documentation/metadata when behavior changes. Preserve user edits. Do not assume a remote, version, release or successful publication. Commit or publish only within the requested scope. Follow `.github/PULL_REQUEST_TEMPLATE.md` for descriptions: problem, resulting behavior, actual validation and deployment/rollback impact.

Before publishing or refreshing a public archive, exclude private env, keys, cache, router exports, USB data, logs, screenshots and build output. Include `.env.example`. `.gitignore` does not protect secrets already tracked: inspect the export/staged files. Never claim the public sample supervisor was tested unchanged on every supported device.

## SMB independence

Keep SMB JSON/account/source policy and supervision independent of the web env, domain, certificates, public NAT and compatibility paths. SMB remains experimental; docs/smb-validation.md records observed native Finder/target smoke checks and unperformed failure/load/reboot checks. Run `make build-smb-mipsle` in addition to the web build and SMB race tests for relevant changes. NT hashes are password-equivalent secrets; never publish actual configs. Only localhost native-client tests are implicitly part of local validation; target writes/execution require authorization.

Explicit SMB disk-root sharing requires `allow_storage_root`, deliberate public-overlap acknowledgment, and descriptor-safe protected-path exclusions. Preserve alias/ancestor-rename protection; a lexical path filter alone is insufficient. Record observed exFAT timestamp limitations without raising process privileges or acknowledging metadata changes that did not occur.

## Structural maintenance

Keep the commands independent. Public HTTP behavior and its tests belong in `internal/web`; startup and configuration stay command-local. Avoid shared privilege/config/supervisor abstractions where ordering and policy differ. Retain upstream SMB source and tests together for auditability; local policy helpers are in `access.go` with regression coverage in `policy_test.go`. Preserve renamed-source provenance in UPSTREAM.md.

CI runs `make check` and `make build build-router`. Existing per-service build targets remain compatible. There is no automatic deployment or release workflow. Follow CONTRIBUTING.md for reviewed source exports; a hygiene check does not replace a secret/content review. Never delete ignored private files, outputs or user data as a repository cleanup.
