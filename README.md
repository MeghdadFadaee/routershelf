# RouterShelf

**Read-only HTTPS file sharing from USB storage on a small Linux router.**

RouterShelf serves a chosen directory with a dark [NAIS](https://github.com/meghdadfadaee/nais) listing, file downloads, copied links, and DPL playlists. It uses a small Go executable, automatic Let's Encrypt certificates, and an optional Padavan-style supervisor. A MikroTik gateway provides public port forwarding and LAN hairpin access.

This repository includes the implementation and the practical guide for configuring, operating, backing up, and removing it. It also preserves an earlier LAN status-dashboard experiment as a standalone example.

## Capabilities

- Directory navigation and file downloads, with HEAD and byte-range support.
- Static pages from the same root: automatic `index.html` and inline HTML/CSS/JavaScript/assets.
- Automatic HTTPS for one configured hostname; HTTP challenge handling and HTTPS redirects.
- Embedded theme assets; no nginx, PHP runtime, database, or frontend build required.
- GET/HEAD only, hidden-path filtering, escaped filenames, and filesystem root confinement.
- Optional USB-aware restart supervision and dedicated IPv4/IPv6 firewall rules on the tested firmware.

The public file service has **no authentication**. HTTPS protects transport; every file placed in the shared directory is available to anyone who can reach the service. Read [security boundaries](SECURITY.md) before deploying.

## Get started

1. Read the [network and storage guide](docs/network-and-storage.md) to identify the gateway, service host, USB mount, and access accounts.
2. Follow [installation and HTTPS setup](docs/getting-started.md), replacing every example value with private deployment settings.
3. Use the [operations guide](docs/operations.md) for health checks, backups, updates, rollback, and removal.

Build and test from the repository root:

```sh
make test
make build-mipsle
```

The cross-build writes `dist/routershelf-linux-mipsle`. The deployment guide explains its installed filename and router paths. Configure deployment without editing source or supervisor code:

```sh
cp .env.example .env
# Edit .env privately, then validate it and print the gateway plan.
ROUTERSHELF_ENV_FILE="$PWD/.env" sh deploy/padavan/supervisor.example.sh --check-config
make nat-plan
```

Read [environment configuration](docs/configuration.md) for available values, precedence, installation, and reloading. `.env` is ignored by Git; `.env.example` is tracked. Gateway changes are still applied manually from the printed plan.

See [static-page behavior](docs/static-pages.md) for index precedence and supported assets.

## Repository layout

```text
cmd/routershelf/             HTTP/HTTPS application and behavior tests
internal/theme/nais/         Licensed NAIS assets embedded in the executable
internal/theme/assets.go     Asset embedding boundary
deploy/padavan/              Environment-driven firmware supervisor
examples/status-dashboard/  Separate legacy BusyBox/Netcat dashboard
docs/                       Setup, operations, architecture, and deployment history
.github/                    CI and issue/pull-request templates
scripts/nat-plan.sh          Gateway plan derived from private environment
.env.example                Public configuration template
Makefile                    Local checks and MIPS little-endian build
project.json                Project identity and compatibility metadata
```

See [architecture](docs/architecture.md) for the separation between application, theme, firmware, and gateway responsibilities. The [deployment history](docs/deployment-history.md) records work completed on the original hardware and what remains unverified. The [status dashboard tutorial](docs/examples/status-dashboard.md) describes the earlier LAN-only example; it is not the public HTTPS server.

## Compatibility and validation

The original deployment used a Neterbit NW-651D with Padavan-derived firmware, Linux 3.4.113, BusyBox 1.24.2, about 128 MiB RAM, and a MIPS little-endian CPU. Its gateway was a MikroTik LHG5 running RouterOS 6.49.22. The tested compiler was Go 1.26.5. Other router firmware, CPU architectures, and newer toolchains require target testing.

On the original device, HTTPS issuance, trusted downloads, HTTP redirects, read-only access checks, theme rendering, and a playlist download were checked. Full reboot, future certificate renewal, sustained load, and authenticated FTP transfer were not verified. The environment-driven supervisor is validated separately; hardware validation is recorded in the deployment history.

## Contributing and publishing

Read [CONTRIBUTING.md](CONTRIBUTING.md). Keep deployment configuration, certificate caches, keys, router exports, USB data, logs, and generated binaries out of Git. All tracked deployment examples use placeholder values. This local repository has no configured remote and has not been published.

Project code is [MIT licensed](LICENSE). NAIS retains its [upstream MIT license](internal/theme/nais/LICENSE). Dependency and theme attribution is collected in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
