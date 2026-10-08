# Third-party notices

## NAIS

Theme CSS, JavaScript, and favicons under `internal/theme/nais` derive from [NAIS](https://github.com/meghdadfadaee/nais). Copyright (c) 2026 Meghdad; MIT license retained in that directory.

Local integration changes add explicit file markers, clipboard completion/error feedback, row alignment, mobile heading wrapping, and a read-only footer. These adaptations do not modify the user's original NAIS checkout.

## Go modules

The application uses `golang.org/x/crypto/acme/autocert` and its Go module dependencies. Versions and checksums are recorded in `go.mod` and `go.sum`. These modules carry their own licenses, commonly the Go project's BSD-style license; consult the license files supplied with each pinned module when distributing binaries.

## Operational downloads

The CA root bundle is obtained separately during deployment. It is not committed to this repository. Review its source and license separately when redistributing it. Runtime certificates and account keys are private deployment material, not source assets.
