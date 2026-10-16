# RouterShelf security boundaries

## Public directory server

Public access is intentionally unauthenticated and read-only. HTTPS encrypts transport; it does not restrict who may download the files. All current and future content in the selected root, including filenames and directory structure, is public.

- Only GET/HEAD are accepted. HTML and recognized web assets are served inline with MIME sniffing disabled; other files remain attachment downloads. This is not an upload API or a server-side application runtime.
- `os.OpenRoot` constrains file access; dot-prefixed paths, traversal components, escaping symlinks, and nonregular files are rejected/omitted. Hidden files are not a substitute for storing secrets outside the root.
- Existing filesystem edge case: regular-file validation happens after opening. On filesystems that support named pipes, a pipe can block the request before it is rejected. Keep special files out of the public root; the observed exFAT target does not support creating these Unix file types. This requires a separate behavior/security fix.
- The application drops privileges to UID/GID 65534. The custom `/_nais/` namespace serves only administrator-embedded theme assets; user files never become executable theme scripts.
- The HTTP fallback redirects to a fixed domain. ACME certificate issuance is limited to the configured domain. TLS validation remains enabled for outbound CA access.
- Timeouts, connection bounds, a listing cap, and a Go memory target reduce resource consumption but are not a complete denial-of-service defense or hard quota. Large traffic can overwhelm a small router or home uplink.
- The private ACME cache, backups, configuration, and binary live outside the public root. On the observed exFAT mount, Unix permission bits do not isolate them from broad FTP/SFTP disk access. Restrict accounts or change storage design for stronger isolation.
- Plain FTP exposes credentials/data to network observers. Prefer SFTP for remote authenticated transfer. A WAN SSH forward expands administrative reachability; keep key authentication and account policy under review.
- IPv6 publication is not implemented. Do not add AAAA records or remove IPv6 protections without a tested design.

The original firmware/kernel are old. This project does not make an unsupported router safe for arbitrary Internet workloads. Keep software and keys maintained, back up privately, and limit publication to intentional public data.

## Before publishing this repository

Use example-only addresses and hostnames. Exclude private config, keys/certificates, cache directories, firmware exports, logs, disk data, screenshots, build outputs, and IDE state. Review staged files. Never paste full router dumps into an issue; provide sanitized diagnostic excerpts.

See [operations](docs/operations.md) for backup, rollback, and removal instructions, and [deployment history](docs/deployment-history.md) for validation limits.

## Optional private SMB service

`routershelf-smb` is an experimental separately configured authenticated service; it does not add authentication or writes to HTTP. It binds a specific private/loopback IPv4 address, requires explicit trusted-source CIDRs, NTLMv2 and signed packets, and drops OS privileges before serving a selected `os.Root` directory. Signing does not encrypt data. Do not expose SMB through WAN forwards/public tunnels or the web firewall chain.

Private JSON contains a password-equivalent NT hash: require protected file modes/ownership on permission-enforcing storage outside all shares. exFAT mount permissions do not isolate service secrets from broadly authorized accounts. Read-only sharing is enforced by the service; per-file ACL changes are unsupported. SMB includes hidden files and allows writes only when explicitly configured. Keep private/public storage separate, list customized public roots, and opt in explicitly to overlap; an upload into a public root can immediately publish its content.

Server-local locks do not coordinate FTP/SFTP/local writers or multiple independent SMB processes. Interrupted writes can leave partial files; exFAT/power loss has no transactional recovery guarantee. Back up data, preserve sources and verify transfers before replacement. Bounds and the Go memory target cannot prevent all resource exhaustion. This backend needs independent security review and target validation before stronger support claims. See [SMB security and limitations](docs/smb.md#privacy-and-filesystem-behavior).

An explicitly enabled disk-root SMB view keeps mandatory service directories and configured protected subtrees hidden and inaccessible. Parent descriptors are pinned with os.Root; symlinks and hard-linked regular files are refused in that view, and protected ancestors cannot be renamed/deleted. `allow_storage_root` and public-overlap acknowledgment are required; default private-folder policy remains unchanged. This application boundary does not isolate secrets from other broadly authorized exFAT accounts.

## Original LAN status dashboard

The BusyBox/Netcat example has no authentication or TLS and runs with the SSH account's privileges. It is suitable only for a trusted, firewall-restricted LAN. Its paths are fixed and never evaluated as shell input, but serial Netcat handling and per-line reads can be monopolized by a hostile client. Do not expose it through WAN forwarding or a public tunnel. This dashboard was removed from the original router.

Configuration is administrator-controlled shell code. Confirm command lines before trusting PID files. Firewall changes are not transactional; verify them after updates and never flush unrelated rules.
