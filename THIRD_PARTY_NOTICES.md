# Third-party notices

## NAIS

Theme CSS, JavaScript, and favicons under `internal/theme/nais` derive from [NAIS](https://github.com/meghdadfadaee/nais). Copyright (c) 2026 Meghdad; MIT license retained in that directory.

Local integration changes add explicit file markers, clipboard completion/error feedback, row alignment, mobile heading wrapping, and a read-only footer. These adaptations do not modify the user's original NAIS checkout.

## Go modules

The application uses `golang.org/x/crypto/acme/autocert` and its Go module dependencies. Versions and checksums are recorded in `go.mod` and `go.sum`. These modules carry their own licenses, commonly the Go project's BSD-style license; consult the license files supplied with each pinned module when distributing binaries.

## Operational downloads

The CA root bundle is obtained separately during deployment. It is not committed to this repository. Review its source and license separately when redistributing it. Runtime certificates and account keys are private deployment material, not source assets.

## Embedded SMB server

`internal/smbwire` derives from [Softwarity smbserver v0.1.1](https://github.com/softwarity/smbserver/tree/v0.1.1), commit `88586e668400859c882428b308408c26086bb1d5`, under Apache License 2.0. The complete license is retained at `internal/smbwire/LICENSE`. This is modified source; [UPSTREAM.md](internal/smbwire/UPSTREAM.md) records provenance and RouterShelf adaptations. Include that license and this attribution when distributing SMB source/binaries. The SMB code uses the existing BSD-licensed `golang.org/x/crypto/md4` dependency; module versions/checksums remain in go.mod/go.sum. Apache-licensed embedded code retains its own license while RouterShelf project code remains MIT.

The protected SMB disk-root view also uses `golang.org/x/sys/unix` (Go project BSD-style license), pinned in `go.mod`/`go.sum`. It supplies descriptor-relative filesystem operations and introduces no router runtime or shared-library dependency.
