# SMB implementation decision

RouterShelf retains its MIT project license and one repository. A second Go executable embeds a narrowly adapted Apache-2.0 SMB implementation; it has no web package imports, certificate logic, public NAT or shared lifecycle. The web executable does not import SMB. Their configuration files, process trees, installed names and runtime directories are separate. SMB uses size-bounded, strictly typed JSON rather than shell configuration: share policy and password-equivalent hashes stay out of environment/argument values, configuration is never executed, and the only application environment input is the independent `RS_SMB_CONFIG` path. The existing web environment contract remains compatible.

## Candidates investigated

| Candidate | Suitability for this initial target |
| --- | --- |
| [Samba](https://www.samba.org/samba/docs/current/man-html/smbd.8.html) | Established implementation with mature authentication, locking and Mac integration. A preferred comparison baseline. A reproducible, maintained static MIPS build with its required dependencies, Linux 3.4 support, exFAT policy and measured combined memory footprint was not established here. Shipping a guessed firmware binary or requiring package installation would not meet this task. GPL obligations would also need a complete corresponding-source distribution plan if bundled. It remains a possible future backend after those checks. |
| [ksmbd](https://www.kernel.org/doc/html/latest/filesystems/smb/ksmbd.html) | Kernel SMB implementation plus userspace utilities, rather than an independent static userspace service for the unmodified 3.4 kernel. Introducing a kernel component is outside this device's constraints. |
| [macos-fuse-t/go-smb2](https://github.com/macos-fuse-t/go-smb2) | Mac-oriented implementation, dual AGPL/commercial licensed. Upstream explicitly identifies authentication, impersonation, parallel connections and locking as early or incomplete. Those are central requirements here; it was not chosen. |
| [go-filesystems/smb](https://github.com/go-filesystems/smb) | Pure Go, BSD-3-Clause, filesystem abstraction with authentication and signing. Young project; would require a separately verified host-filesystem driver and bounded positional I/O throughout. Not integrated during this change. |
| [softwarity/smbserver v0.1.1](https://github.com/softwarity/smbserver/tree/v0.1.1) | Chosen as an experimental implementation base: pure Go, existing pinned x/crypto dependency, os.Root, NTLMv2, required signing, source filtering, read-only mode, real-socket/fuzz tests and reported native-client CI. One user/share scope is explicit. Upstream defaults and ephemeral metadata are inappropriate for this router, so a local source adaptation is maintained with its license and provenance. Upstream CI claims are not RouterShelf validation. |

This choice favors a locally buildable executable with inspectable policy over introducing deployment dependencies. It does **not** establish production readiness or substitute for a security audit. SMB is a large protocol; the young backend and local modifications need continued maintenance, fuzzing, independent review and real-client coverage. See [upstream adaptation inventory](../internal/smbwire/UPSTREAM.md).

## Resource and persistence decisions

The binary targets Go 1.26, Linux/MIPS LE soft-float, CGO disabled. Kernel availability, RSS, throughput, CPU impact on routing, disk failure behavior and boot recovery must be measured on the actual router after approval. No performance estimate is inferred from desktop tests or upstream benchmarks.

Protocol allocations are bounded by request size, connections and handles, not file length. Fixed low limits and a 20 MiB GC target leave room for testing alongside the web service, but cannot guarantee a combined hard quota. Exceeding directory limits returns an error rather than silently omitting names. Server-local sharing checks and byte-range locks protect cooperating SMB clients; external writers bypass those checks. SMB2 packet signing is mandatory, but there is no SMB encryption, so network privacy remains a LAN/VPN responsibility.

exFAT metadata is represented by ordinary disk sidecars where supported by the client, not acknowledged into temporary RAM streams. Do not promise persistent ACLs, Finder tags, resource-fork compatibility, Time Machine or application-specific safe saving until those paths are verified. Close/flush/write-through failures are reported. Transfer interruption and power loss still require file verification and backups; this is not an atomic upload API.

## Maintenance and distribution

Keep upstream source provenance, retained tests and Apache license together; review both upstream fixes and local policy changes when upgrading. Protocol source is embedded under `internal/smbwire`, not fetched at deployment. `go.mod`/`go.sum` remain compatible with the existing web dependencies. Distributors must include the project MIT license, NAIS license where distributing web assets, SMB Apache-2.0 license and Go-module license notices. No binaries have been published by this change.

Never ship real JSON config or NT hashes. Include only `.smb.example.json`, `.env.example`, source, licenses and documentation in public source archives. Generated `dist/` executables are local validation artifacts and must be packaged separately with their notices if a binary release is later authorized. Inspect archive contents; Git ignore rules alone are insufficient.
