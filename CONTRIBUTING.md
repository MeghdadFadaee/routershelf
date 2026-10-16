# Contributing to RouterShelf

## Development

Use Go 1.26 or a newer toolchain verified against the target kernel, and Python 3.10+ for the stress-runner tests. From the repository root, run `make check` and `make build build-router`. Keep changes focused and describe user-visible behavior and validation. A desktop cross-build proves compilation, not target-kernel compatibility or successful boot persistence.

HTTP response behavior and its tests belong in `internal/web`; web process startup/configuration belongs in `cmd/routershelf`. SMB startup/configuration/storage monitoring stays in `cmd/routershelf-smb`, separate from its adapted protocol implementation in `internal/smbwire`. Theme changes belong in `internal/theme/nais`; retain the upstream license and document local adaptations. Padavan-specific lifecycle rules belong in `deploy/padavan`. Keep the legacy status dashboard independent under `examples/status-dashboard`.

Run the web supervisor’s `--check-config` against a trusted private env to validate without touching firewall, processes, or the disk. Do not run deployment scripts on a device merely to test formatting. Inspect the real mount, firewall, interface names, and firmware hooks before any target installation. The example supervisor is administrator-controlled shell code, not an untrusted input format.

## Verification

For file-serving changes, check scope confinement, filename escaping, GET/HEAD, rejected write methods, and byte-range behavior. For TLS changes, verify trusted hostname validation, redirects, certificate-cache retention, and challenge routing. For UI changes, verify folder navigation, download, copied links, playlists, and mobile layout.

Record which checks actually ran. Do not claim a reboot or certificate renewal passed unless it was observed. Include a rollback plan for changes that alter installed paths, supervision, or firewall rules.

## Public data hygiene

Never attach private keys, certificate caches, full router exports, USB files, usernames/passwords, public-IP deployment values, or unredacted logs/screenshots. Use the example subnet, documentation IP range, and `files.example.com` in public issues. A sanitized minimal configuration is more useful than a full device dump.

Before publication, review staged files and metadata and verify the intended repository identity. There is no automatic deploy or publish action in this project.

Stress-runner changes should be tested with `make test-stress-tool` against localhost and synthetic telemetry, not by loading the live deployment. Only run a live workload when requested, with the target and bounds clear. See [stress testing](docs/stress-testing.md).

## SMB contributions

Keep `cmd/routershelf-smb` independent of web configuration/certificates/public exposure. Protocol code in `internal/smbwire` is an adapted Apache-2.0 dependency; preserve its license, provenance and adaptation inventory. Run `make test`, `go test -race ./cmd/routershelf-smb ./internal/smbwire`, and `make build-mipsle build-smb-mipsle`. Add meaningful authentication/signing, read-only, confinement, sharing/locking, file mutation, interruption and bounded-resource tests for affected behavior. Never replace disk metadata with successful volatile writes.

Native macOS checks are opt-in: use `make build` to build the desktop SMB executable and run `python3 -B scripts/test_smb_macos.py /absolute/path/to/routershelf-smb`; it mounts only a temporary localhost share with synthetic credentials. Repeat with `--read-only` to check native mutation refusals and `--disk-root` for the explicit protected root view. Native Finder GUI, actual exFAT, router kernel, resource/coexistence, USB loss, reboot and interrupted native-client tests are separate checks and must be recorded honestly. Read-only router inspection does not authorize executing a new binary or writing test data there. Keep real `smb.json`, hashes and logs private.

## Local workflow and source distribution

`make test` runs the routine suite, including localhost stress-runner tests and disposable-Git-tree checks for the repository checker. `make check` adds formatting, Git-visible path/documentation/metadata checks, `go vet`, and race tests for all packages; CI uses the same target. `make build` builds both native commands, and `make build-router` builds both static Linux/MIPS LE soft-float commands. The original per-service cross-build targets remain available. Native macOS mounting and any live/device checks are opt-in and separate from CI.

Builds and diagnostics belong in ignored `dist/` and `outputs/`. Do not delete private settings, operator notes or data during cleanup. Keep the useful standalone dashboard tutorial under `examples/`; it is not a third RouterShelf service. No dependency update is implied by structural refactoring.

For an authorized source export, review a committed revision with `git ls-tree -r --name-only REVISION`, including templates, licenses and third-party notices. Export that reviewed revision with `git archive`; do not zip the working directory. `git archive` excludes untracked/ignored data but still includes any secrets committed in that revision, so review contents before sharing. Build outputs are packaged separately only when a binary distribution is requested, with all applicable project, embedded-source and pinned-module licenses. See [third-party notices](THIRD_PARTY_NOTICES.md). No release packaging or publication is performed by a build target.
