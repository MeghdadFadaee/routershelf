# Architecture

RouterShelf has four operational layers:

| Layer | Responsibility | Repository location |
| --- | --- | --- |
| Go application | Safe directory reads, HTTP responses, TLS and ACME, connection limits, privilege drop | `cmd/routershelf` |
| Embedded theme | NAIS styling, favicon, copied links, playlist generation | `internal/theme` |
| Router lifecycle | USB checks, process restart, time startup, host firewall hooks, flash persistence | `deploy/padavan` |
| Network gateway | Public DNS destination, destination NAT, LAN hairpin reply routing | Configuration documented in `docs/getting-started.md` |

```text
Browser -> gateway TCP 80/443 -> storage host TCP 10080/10443
                                       |
                                  RouterShelf
                                  /         \
                       shared directory    private ACME cache
                                  |
                             USB filesystem
```

The gateway plan is generated from `.env` by `scripts/nat-plan.sh`, rather than auto-applied: changing a gateway requires matching its actual interfaces, existing NAT/filter rules, and public address. The Go application does not change routing, configure DHCP, or provision FTP accounts.

The `deploy/padavan` script loads a private environment file for one firmware family. Its installed names remain `public-share-service.sh`, `public-directory`, and `/tmp/public-share-*` for compatibility with the existing deployed system and instructions. Renaming the repository is not a live-router migration.

The legacy dashboard is a separate Netcat/shell example, with its own configuration and security assumptions. Its code remains in `examples/status-dashboard` and its guide in `docs/examples/status-dashboard.md`. It was removed from the original router and must never inherit the public server's port-forwarding policy.

The Go module path is currently `routershelf`, a local standalone module identity. If a real repository remote is established later, change the module path and the `routershelf/internal/theme` import together if a hosted module path is desired. No remote namespace is assumed.

Read [setup](getting-started.md), [operations](operations.md), and [security](../SECURITY.md) for configuration and maintenance boundaries.
