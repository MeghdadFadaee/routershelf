# Architecture

RouterShelf is one Go module with two independent commands. Each owns its configuration and process lifecycle. Small command-local files keep startup concerns together; only the HTTP response layer and the substantial SMB protocol implementation need separate internal packages.

| Component | Responsibility | Location |
| --- | --- | --- |
| Web command | Environment/flag validation, privilege drop, listeners, connection bounds, TLS and ACME | `cmd/routershelf` |
| Public HTTP | Filesystem confinement, methods, listings, static pages, download headers and HTTPS redirects | `internal/web` |
| Embedded theme | NAIS CSS/JavaScript, favicon, copied links and playlists | `internal/theme` |
| SMB command | Private JSON validation, credentials CLI, listener/privilege setup, mount/root monitoring | `cmd/routershelf-smb` |
| SMB protocol | Authentication, signing, permissions, bounded operations, locks and protected storage view | `internal/smbwire` |
| Router lifecycle | Independently runnable firmware supervisors; web firewall/time hooks | `deploy/padavan` |
| Gateway planning | Print administrator-reviewed public web NAT guidance; never apply it | `scripts/nat-plan.sh` |

## Dependencies and access policy

```text
routershelf command -> internal/web -> internal/theme
                  -> x/crypto/acme/autocert

routershelf-smb command -> internal/smbwire -> x/crypto, x/sys
```

Neither executable imports the other service. There is no shared configuration or lifecycle abstraction: web environment/shell policy and SMB protected JSON have different compatibility and trust contracts. Similar-looking privilege handling also has different ordering: web drops before binding unprivileged ports; SMB binds its private listener before dropping so it can use TCP 445.

The HTTP handler opens an `os.Root` per request and preserves current removable-storage behavior. SMB pins its selected root for a session-serving lifetime and exits when mount/root identity changes. Its protected disk-root view adds descriptor-based exclusions; see the [adaptation inventory](../internal/smbwire/UPSTREAM.md). Both may use the same physical disk, so resource pressure and explicitly overlapping data remain shared operational risks.

## Deployment boundaries

Public web traffic can follow `Browser -> gateway TCP 80/443 -> storage-host HTTP/HTTPS listeners`. Certificate/account cache, binary and configuration belong outside the public root. The [configuration reference](configuration.md) describes the environment contract.

Private access follows `Finder -> specific private IPv4 SMB listener -> selected USB directory`. It requires no domain, certificates, NTP startup or gateway forwarding. The SMB supervisor uses its own executable/config paths and `/tmp/routershelf-smb/` lock. It does not source the web environment, change firewall rules or signal the web service. Administrators restrict the listener with their own service-scoped firewall policy; see [SMB setup](smb.md).

The web supervisor's installed compatibility names remain `public-share-service.sh`, `public-directory` and `/tmp/public-share-*`. Repository organization does not migrate deployed installations. Padavan hooks, flash persistence and BusyBox details remain platform-specific and should not become application responsibilities. The Go commands do not provision DHCP, alter routing/NAT or create FTP accounts.

The module path remains `routershelf`; no hosted namespace is assumed. The [legacy dashboard](../examples/status-dashboard/README.md) is a standalone historical tutorial with separate security assumptions and no Go dependency. It must not inherit public forwarding.

Read [web setup](getting-started.md), [web operation](operations.md), [SMB setup and operation](smb.md), and [security](../SECURITY.md) for user procedures. Validation history records observations; it is not the installation source of truth.
