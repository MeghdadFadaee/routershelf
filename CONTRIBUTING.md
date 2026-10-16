# Contributing to RouterShelf

## Development

Use Go 1.26 or a newer toolchain verified against the target kernel, and Python 3.10+ for the stress-runner tests. From the repository root, run `make test` and `make build-mipsle`. Keep changes focused and describe user-visible behavior and validation. A desktop cross-build proves compilation, not target-kernel compatibility or successful boot persistence.

Application changes belong in `cmd/routershelf`. Theme changes belong in `internal/theme/nais`; retain the upstream license and document local adaptations. Padavan-specific lifecycle rules belong in `deploy/padavan`. Keep the legacy status dashboard independent under `examples/status-dashboard`.

Run `--check-config` against a private env to validate without touching firewall, processes, or the disk. Do not run deployment scripts on a device merely to test formatting. Inspect the real mount, firewall, interface names, and firmware hooks before any target installation. The example supervisor is administrator-controlled shell code, not an untrusted input format.

## Verification

For file-serving changes, check scope confinement, filename escaping, GET/HEAD, rejected write methods, and byte-range behavior. For TLS changes, verify trusted hostname validation, redirects, certificate-cache retention, and challenge routing. For UI changes, verify folder navigation, download, copied links, playlists, and mobile layout.

Record which checks actually ran. Do not claim a reboot or certificate renewal passed unless it was observed. Include a rollback plan for changes that alter installed paths, supervision, or firewall rules.

## Public data hygiene

Never attach private keys, certificate caches, full router exports, USB files, usernames/passwords, public-IP deployment values, or unredacted logs/screenshots. Use the example subnet, documentation IP range, and `files.example.com` in public issues. A sanitized minimal configuration is more useful than a full device dump.

Before the first publication, review staged files and metadata, configure the actual Git remote, and replace publication placeholders where needed. There is no automatic deploy or publish action in this project.

Stress-runner changes should be tested with `make test-stress-tool` against localhost and synthetic telemetry, not by loading the live deployment. Only run a live workload when requested, with the target and bounds clear. See [stress testing](docs/stress-testing.md).

## SMB contributions

Keep `cmd/routershelf-smb` independent of web configuration/certificates/public exposure. Protocol code in `internal/smbwire` is an adapted Apache-2.0 dependency; preserve its license, provenance and adaptation inventory. Run `make test`, `go test -race ./cmd/routershelf-smb ./internal/smbwire`, and `make build-mipsle build-smb-mipsle`. Add meaningful authentication/signing, read-only, confinement, sharing/locking, file mutation, interruption and bounded-resource tests for affected behavior. Never replace disk metadata with successful volatile writes.

Native macOS checks are opt-in: build the desktop SMB executable and run `python3 -B scripts/test_smb_macos.py /absolute/path/to/routershelf-smb`; it mounts only a temporary localhost share with synthetic credentials. Repeat with `--read-only` to check native mutation refusals. Native Finder GUI, actual exFAT, router kernel, resource/coexistence, USB loss, reboot and interrupted native-client tests are separate checks and must be recorded honestly. Read-only router inspection does not authorize executing a new binary or writing test data there. Keep real `smb.json`, hashes and logs private.
