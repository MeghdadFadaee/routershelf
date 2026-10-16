# RouterShelf

USB file sharing for small Linux routers, with two independently runnable Go services:

| Service | Access | Start here |
| --- | --- | --- |
| `routershelf` | Public, unauthenticated, read-only HTTP/HTTPS | [Web setup](docs/getting-started.md) |
| `routershelf-smb` | Private, authenticated SMB for Finder; read-only by default, optional writes | [SMB setup](docs/smb.md) |

Run either service or both. SMB requires no web configuration, public domain, or certificates. Each service has its own configuration, account policy, supervisor, and update/recovery procedure.

The web service provides [NAIS](https://github.com/meghdadfadaee/nais) directory listings, downloads, copied links, DPL playlists, static pages, HEAD/byte ranges, and automatic HTTPS for one hostname. The experimental SMB service lets Finder browse and manage explicitly selected storage using macOS's built-in client.

**The web root is public.** HTTPS protects transport but adds no authentication. Keep private SMB folders separate by default; deliberately overlapping storage can publish uploaded content immediately. Read [security boundaries](SECURITY.md) before installation.

## Setup and operation

For web sharing, follow [network and storage preparation](docs/network-and-storage.md), then [installation](docs/getting-started.md). Configure a private copy of [`.env.example`](.env.example); [configuration](docs/configuration.md) covers environment values, flags and reload. The [operations guide](docs/operations.md) covers health checks, troubleshooting, updates, rollback and removal. [Static pages](docs/static-pages.md) explains index precedence and inline assets.

For Finder, follow the [SMB guide](docs/smb.md): build, choose storage, create credentials, configure a private copy of [`.smb.example.json`](.smb.example.json), and install its independent supervisor. The same guide covers permissions, status, troubleshooting, updates and removal. SMB is private by default and must not inherit public web forwarding.

The Padavan deployment examples are optional and firmware-specific. A gateway such as MikroTik can forward public HTTP/HTTPS; a gateway is not needed for local SMB. No runtime, compiler, package installation, or shared library is required on the router.

## Development

Use Go 1.26 and Python 3.10+ on the development computer. From the repository root:

```sh
make test          # Go tests, shell/config checks, Python tool tests
make check         # Also formatting, docs/source hygiene, vet and race tests
make build         # Native dist/routershelf and dist/routershelf-smb
make build-router  # Both static Linux/MIPS LE soft-float executables
```

The existing `make build-mipsle` and `make build-smb-mipsle` targets remain available independently. Builds go under ignored `dist/`; private configurations and local reports stay ignored. Nothing here deploys automatically. See [contributing](CONTRIBUTING.md) for validation, native-client checks and distribution guidance.

## Code and documentation

```text
cmd/routershelf/       Web configuration, startup, TLS and connection limits
cmd/routershelf-smb/   SMB configuration, startup and storage supervision
internal/web/         Public HTTP handlers, listings, static files and tests
internal/theme/       Embedded NAIS assets and their license
internal/smbwire/     Adapted SMB protocol implementation, license and tests
deploy/padavan/       Independent firmware supervisor examples
scripts/              Local checks, gateway planning and test tools
docs/                 User guides, architecture and validation records
examples/             Separate historical LAN status-dashboard tutorial
```

Read [architecture](docs/architecture.md) for responsibilities and dependency boundaries. [Web deployment history](docs/deployment-history.md) and [SMB validation](docs/smb-validation.md) distinguish observed results from remaining hardware checks. The [SMB implementation decision](docs/smb-implementation.md) records backend tradeoffs and maintenance obligations. [Stress testing](docs/stress-testing.md) describes the opt-in bounded load tool; local tests do not authorize live load.

## Compatibility and limitations

Observed target: Padavan-derived firmware, MediaTek MT7621 MIPS little-endian, Linux 3.4.113, BusyBox 1.24.2, approximately 128 MiB RAM, no swap, and exFAT USB storage. The recorded toolchain is Go 1.26.5. Other firmware, architectures and toolchains require target testing.

Web HTTPS and read-only operations have been observed on that target. SMB authentication, protected disk-root access, small checksum-verified transfers, Finder copy/Quick Look and independent restart have also been observed. exFAT refused arbitrary timestamp restoration by the non-root SMB account. SMB remains experimental; full reboot, USB-loss recovery, large native transfers, complete Mac metadata compatibility and sustained combined load remain unverified. Compilation and desktop tests alone do not prove device compatibility.

## License and distribution

RouterShelf project code is [MIT licensed](LICENSE). Embedded SMB code retains [Apache-2.0](internal/smbwire/LICENSE); NAIS retains its [MIT license](internal/theme/nais/LICENSE). Preserve [third-party notices](THIRD_PARTY_NOTICES.md) and module license obligations when distributing.

Keep configuration, NT hashes, passwords, certificates, router exports, USB data and diagnostics out of source archives. Read [contribution and distribution guidance](CONTRIBUTING.md) before sharing source or binaries. Project identity and compatibility metadata are in [project.json](project.json); no remote or release is assumed.
