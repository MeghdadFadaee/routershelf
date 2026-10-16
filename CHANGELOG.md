# Changelog

## Unreleased

### Structure and maintenance

- Separate public HTTP behavior/tests into `internal/web`, with command-local startup and configuration.
- Organize SMB CLI, service loop and storage/privilege operations into focused command-local files; name local protocol policy helpers by responsibility.
- Add shared local/CI check targets, native and combined router builds, and public-source/documentation consistency checks.
- Clarify independent setup paths and current validation limits. CLI, configuration formats and deployed installation paths remain compatible.

### Private SMB

- Add the independent experimental authenticated SMB executable, private JSON configuration and USB-aware supervisor.
- Support explicit read-only/read-write sharing, signed sessions and an opt-in protected disk-root view.
- Record local/native and authorized target smoke results, exFAT timestamp limitations and remaining failure/load/reboot checks.

### Stress testing

- Added a bounded read-only Python runner with phased concurrency, rate caps, optional router telemetry and stop thresholds.
- Added keep-alive/fresh-TLS modes, static page/range/redirect preflight checks and private JSON/CSV/Markdown reports.
- Verified the runner locally; no live-router stress load has been run.

### Static pages

- Render directory `index.html` and direct HTML requests from the existing shared root.
- Serve recognized web assets inline while retaining attachment downloads for other files.
- Keep read-only, path confinement, HTTPS and NAIS fallback behavior.

### Environment configuration

- Added a private `.env` and tracked `.env.example` for network, domain, USB, port, time, and runtime values.
- Go flags override environment defaults; the supervisor loads the same private configuration.
- Added side-effect-free config checks and a manually applied gateway NAT plan.
- Removed the public-mode marker dependency and reconciled owned firewall rules from configuration.

### Repository organization

- Named the project RouterShelf and moved the Go module to the repository root.
- Separated the application, embedded theme, Padavan deployment example, and legacy status dashboard.
- Added project metadata, architecture documentation, contribution guidance, third-party notices, local build commands, and CI templates.

### Included implementation

- Read-only USB directory listing and downloads, with constrained filesystem access.
- Automatic HTTPS certificate management, HTTP redirect/challenge handling, and persistent cache support.
- NAIS theme, copied links, DPL playlists, and clipboard feedback.
- Example USB supervisor, firewall hooks, and network/storage operations guides.

These entries describe repository content; they do not announce a published release or new changes to the live router.
