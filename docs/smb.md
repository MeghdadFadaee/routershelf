# Finder access with RouterShelf SMB

RouterShelf has two independent executables. `routershelf` publishes unauthenticated read-only web content. `routershelf-smb` provides authenticated private storage access through Finder's built-in SMB client. Either works alone. SMB requires no web domain, certificates, web environment file, gateway forwarding, Mac extensions, or additional Mac software.

**Experimental:** local protocol tests and cross-compilation are separate from native-client and hardware validation. See [SMB validation](smb-validation.md) for actual results. Back up valuable data before enabling writes. This release supports one selected directory, one account, and one read-only/read-write policy per instance. It is not a multi-user ACL server, Time Machine target, domain server, or highly available NAS.

## Choose private storage

Use separate sibling directories on USB, for example:

```text
/media/DISK_ID/private-storage/      authenticated SMB files
/media/DISK_ID/public-storage/       deliberately public web files
/media/DISK_ID/.smb-service/         executable and private update backups
/etc/storage/routershelf-smb.json    administrator-owned credentials/config
```

Replace `DISK_ID` with your actual mount. Create the selected private directory deliberately while the disk is mounted. The service never creates a missing root or serves the whole mount. Never share `/`, the USB mount itself, a service directory, certificate cache, backup directory, or administrator home. Select one root containing the folders you want available; every descendant, including hidden files, is available to the authenticated account. Symlinks cannot escape that root. On filesystems supporting hard links, an administrator must also exclude hard links to sensitive files.

`public_roots` must list every web root on this storage, especially a customized `RS_ROOT`. The standard `public-storage` sibling is checked automatically. Default private service directories, this executable and the config are excluded; add customized certificate/cache/binary/backup paths to `protected_paths`. These exclusions cannot be overridden by allowing public overlap. Overlap is refused unless `allow_public_overlap` is explicitly true. Separate processes cannot discover arbitrary web installations: listing the actual public roots is the administrator's responsibility. If you deliberately share a web root, **every SMB upload/rename into it may immediately publish the file without web authentication**. Use a private staging directory and move reviewed files into publication deliberately. The web service retains its own existing path filtering and GET/HEAD policy; SMB credentials do not protect web copies.

exFAT has mount-wide ownership/mode behavior, limited Unix metadata and no POSIX ACL isolation. A separate non-root service UID reduces process privilege but does not isolate secrets on a broadly writable exFAT mount. Put credentials on a permission-enforcing administrator-owned filesystem such as `/etc/storage`; do not rely on `chmod` on exFAT. Restrict other disk accounts and local writers separately. Use only one RouterShelf SMB instance per writable root; independent instances and FTP/SFTP/local programs do not share its locks.

## Build and configure

On the development computer, with Go 1.26+:

```sh
make test
make build-smb-mipsle
# For a local desktop executable:
go build -o dist/routershelf-smb ./cmd/routershelf-smb
umask 077
cp .smb.example.json smb.json
chmod 600 smb.json
```

Edit private `smb.json`. The sample intentionally has an unusable hash and placeholder paths. Keep its mode 0600. On macOS use a hidden terminal prompt to generate a password-equivalent NT hash without putting the password in arguments or shell history:

```sh
read -r -s 'smb_password?New unique SMB password (12+ characters): '
printf '\n'
printf '%s' "$smb_password" | ./dist/routershelf-smb -hash-password
unset smb_password
```

Paste the output into `nt_hash` privately. Do not reuse a router/admin password. The hash is as sensitive as the password: anyone holding it can authenticate. Do not put it in issues, screenshots, logs, archives or backups intended for publication. Finder receives the original password; the server verifies NTLMv2 challenge responses using the hash. There is no guest, anonymous or empty-password mode.

For LAN access set `listen` to the router's specific private IPv4 and port 445; put your trusted client subnet(s) in `allow`. `127.0.0.1:1445` and `127.0.0.1/32` are safe local defaults. Start read-only. Set `read_only` to false only after verifying the selected root and backup. Use a dedicated available non-root UID/GID if supported by the mount; the defaults are 65533, distinct from the web service’s 65534; verify that this ID is available on your firmware. Root is used only to read protected config, bind a privileged port and drop supplementary groups/UID/GID before serving files. An already unprivileged process must match both configured IDs and use an available unprivileged port. Do not disable privilege dropping.

Check on the machine with the real paths (the computer cannot validate a router-only USB path):

```sh
./dist/routershelf-smb -config /absolute/private/smb.json -check-config
```

Validation does not listen, write files, change firewall or check password strength from its hash. It requires existing paths and a protected regular config file. On Linux the running service additionally requires `storage_mount` to be an exact `/proc/mounts` mountpoint. On a desktop Mac this mountpoint requirement is omitted for local testing.

## Configuration reference

The only environment setting is `RS_SMB_CONFIG`, the absolute JSON config path. `-config` overrides it; `-check-config` validates and exits. No web `RS_*` setting is consulted, and no shell file is sourced. Unknown JSON fields, extra objects, and configs above 32 KiB are rejected without echoing input. Change configuration by restarting only SMB. Credentials are read once at startup.

| JSON field | Default / contract |
| --- | --- |
| `listen` | `127.0.0.1:1445`; a specific private/loopback IPv4 and numeric port; no wildcard, DNS, public IPv4 or IPv6 |
| `storage_mount` | Required existing absolute USB mount; exact Linux mount presence checked before listening and every two seconds |
| `root` | Required existing directory strictly under that mount; selected explicitly; never created automatically |
| `share` | `Storage`; direct Finder share name; no separators/spaces or reserved `IPC$` |
| `user` | Required single account name; no OS account/password database needed |
| `nt_hash` | Required 32 hex digits; placeholders and empty-password hash refused |
| `read_only` | `true`; refuses data, directory, metadata, rename and delete mutations |
| `allow` | Required explicit private/loopback IPv4 CIDRs; applied before authentication; no unrestricted prefixes |
| `protected_paths` | Additional sensitive paths that must not overlap the share; default service directories and running executable are also excluded |
| `public_roots` | Additional actual public roots; standard `storage_mount/public-storage` always checked |
| `allow_public_overlap` | `false`; deliberate opt-in to overlapping publication |
| `uid`, `gid` | 65533; positive non-root IDs, applied before file service starts |
| `diagnostics` | `false`; opt-in command/status/credit and peer-event logging without filenames or NTLM tokens; use bounded private logs |
| `case_insensitive` | `true` for exFAT; use false for a case-sensitive filesystem |

Paths must be clean and absolute. Root/config symlinks are resolved during validation. Existing inaccessible public-root paths cause validation failure. Private config cannot reside inside the share. The web `.env.example`, deployed binary name, environment precedence and `/tmp/public-share-*` paths remain unchanged.

## Finder

With SMB running, in Finder choose **Go → Connect to Server** (⌘K):

```text
smb://ROUTER_LAN_IP/Storage
```

Replace `ROUTER_LAN_IP`; do not type that placeholder literally. For an explicitly configured high port, use `smb://ROUTER_LAN_IP:1445/Storage`. Enter the configured account and original password as a registered user. Use the complete share URL: automatic network discovery and browsing a server's share list are not implemented. Save to Keychain only on a trusted Mac. Eject the share before service updates or removing USB storage.

Browse/open/download in either mode. With writes enabled you can upload, create folders, rename, move within the share and delete. Moving across shares/filesystems is a client copy then deletion and is not atomic. Deletions are permanent; this service has no recycle bin or snapshots. Finder may keep cached views until refresh/reconnect. After a password change, eject and replace stale Keychain credentials.

## Privacy and filesystem behavior

SMB2.0.2/2.1 (SMB3.0 fallback) uses NTLMv2 and mandatory packet signing. **Signing does not encrypt filenames or contents.** Use a trusted isolated LAN, or an independently secured VPN for access from elsewhere. Never forward TCP 445/1445, enable a public tunnel, add SMB to the web NAT plan, or reuse the web service's public firewall allowance. SMB binds IPv4 only and checks peer source prefixes; a source-masquerading proxy can defeat source-based distinctions, so also keep gateway and host firewall exposure private.

The service changes no firewall rules. If existing firewall policy blocks LAN SMB, an administrator must allow only the chosen IPv4/interface/client subnet and service port, with an explicit drop for other sources to that port. Use a separately owned SMB chain/hook, preserve SSH and unrelated rules, and test denied guest/WAN sources. Do not flush tables or permit SMB through the existing public web chain. If firmware already listens on 445, leave it alone and select a high port for this initial service. Firewall/persistence changes need separate deployment approval and firmware-specific review.

The exFAT profile advertises case-insensitive, case-preserving names. There are no persistent named streams or ACLs. Per-file read-only/locked mode changes are refused rather than acknowledged on a mount that may ignore them; use the share’s `read_only` policy. Finder can create `._filename` AppleDouble sidecars and `.DS_Store` files on disk; retain these with their files if metadata matters. Some applications may require streams, ACLs, resource forks or Apple extensions that this profile cannot provide. Unsupported requests return errors; no metadata is acknowledged into volatile streams. Finder tags, resource-fork round trips, packages and application-specific saves still require native testing. Do not clean sidecars automatically. Files visible through SMB include dotfiles; the web service still filters them.

Writes use bounded 64 KiB chunks, not whole-file memory. Four connections, 64 total handles, eight compound commands and 1,024 entries per scanned directory bound work. Larger directories return a resource error, rather than an incomplete successful listing; divide large collections into folders. Authentication has a 30-second per-frame deadline and authenticated reads a five-minute idle deadline; reconnect idle clients as needed. The Go memory target is 20 MiB, **not a hard quota**. Protocol state, filesystem caches, firmware and the web process need additional RAM; target resource measurements remain required before enabling both under load.

SMB share access checks and byte-range locks coordinate clients of this instance; unlocked applications may still overwrite each other by design. Writes acknowledge OS writes, honor write-through, and synchronize writable handles on explicit close/flush; errors are returned. OS/device caching, exFAT and power failure can still lose data. A network cut may leave a partial destination file or an uncertain last write; it is not a transaction and does not roll back an overwrite. Keep the source, retry to a distinct temporary name, compare length/checksum and then rename into place. No durable handles, transactional uploads, crash recovery, or automatic resume are promised. Never unplug a disk during writes; back up and check it after an unsafe disconnect.

The executable closes sessions and exits when the root identity or mount disappears/changes (poll interval two seconds). It retains an `os.Root` descriptor rather than falling back to a RAM directory. A kernel/device I/O hang can delay shutdown; do not force router reboots or kill unrelated services as an automatic recovery action. The supervisor retries after ten seconds; it never initializes a missing disk. Finder may reconnect by path; failed copies must still be verified and retried.

## Install, status, update and remove

These are procedures for an approved deployment, **not actions performed by the repository implementation**. First inspect firmware USB/startup hooks, current port listeners, mount ownership/free space, available UID/GID and firewall policy read-only. Do not stop firmware Samba or any web process. Build the static Linux/MIPS little-endian soft-float executable; no router compiler/runtime/library/package installation is needed. Cross-compilation alone does not prove Linux 3.4 compatibility.

1. Back up USB data, and the existing SMB binary/config/script privately if present. Eject any connected Finder share. Keep binaries/backups outside both shared roots.
2. Upload the binary using SCP to a new filename such as `routershelf-smb.new` in the private service directory. Upload config to a new protected file on permission-enforcing storage, and the supervisor to a new filename. Inspect permissions and checksums. Do not overwrite a running binary in place.
3. Validate the new binary with `-config /absolute/private/routershelf-smb.json.new -check-config` on the router. This only reads configuration and paths. Run target smoke/Finder checks only with approval. Preserve the previous names privately, then rename the validated files into their installed paths on their respective filesystems.
4. For foreground startup, export `RS_SMB_BINARY` (absolute installed executable) and `RS_SMB_CONFIG` (absolute private JSON), then run `sh /absolute/path/routershelf-smb-service.sh`, installed from `deploy/padavan/smb-supervisor.example.sh`. It needs neither web supervisor nor web environment. Observe the ready/error message; logs can contain private diagnostics, so do not publish them. For background operation route output to an existing bounded private firmware log facility (for example BusyBox `logger`), not an unbounded RAM log.
5. Only after successful manual checks, add a separate invocation with those two variables to the verified USB-aware hook. Do not replace existing web, Wi-Fi, NTP or SSH hooks. Save changed `/etc/storage` config/scripts/hooks using the firmware's normal persistence action only when needed; USB binary updates alone do not require a flash save. Full reboot behavior remains a target check.

The supervisor owns `/tmp/routershelf-smb/` exclusively; `supervisor.pid` and `child.pid` are diagnostic, never authorization to signal a process. Inspect `/proc/PID/cmdline` before sending SIGTERM to the supervisor. It signals/waits only for the child it launched and removes its runtime lock. Wait for cleanup before starting one replacement. After a crash or SIGKILL a stale lock refuses startup; verify both processes are gone, then remove only the stale SMB runtime files/directory. Do not remove live lock directories or web PID files.

For status check the verified supervisor/child command lines, UID/GID and `VmRSS` in `/proc/PID/status`, exact mount, free space and logs, then try Finder login and a read. A PID is not a readiness guarantee; a waiting supervisor may have no child while storage/config is unavailable. Do not include full config, hash, private filenames or raw logs in a public support request.

For updates, stop only the verified SMB supervisor and wait for child cleanup, rename a checked new binary/config into place, restart only SMB and verify login/file checks. To roll back, stop SMB, restore the known-good private binary/config/script, restart it and check again. Keep the same data root and data; binary rollback cannot undo deleted/overwritten files. Restore user data from backup separately if necessary. The certificate cache and web service need no change.

For removal, eject Finder shares, remove only the SMB hook invocation, stop the verified SMB supervisor, wait for child/lock cleanup, remove its executable/config/script and any separately owned SMB firewall rules. Save flash once if persistent hooks/config changed. Keep USB data and private backups until intentionally retired. Web installation paths, startup, NAT, certificates and sharing continue independently.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Cannot connect | Exact private IPv4, port, complete share URL, USB availability, matching `allow`, narrow host firewall policy, another listener on 445 |
| Login fails | Registered user, original password, correct NT hash, stale Keychain entry; no guest mode |
| Read works but upload fails | `read_only`, disk full/read-only mount, UID/GID mount permissions, directory entry/handle/lock limits |
| Config validation fails | Protected regular file, absolute existing paths, replaced hash placeholder, unknown fields, public overlap |
| Supervisor runs without child | Missing executable/config/storage or repeated startup errors; inspect private diagnostics |
| Finder metadata/app save fails | No native streams/ACLs/AAPL/leases; keep sidecars; test that application on a scratch share |
| Copy interrupted | Retain source, reconnect, inspect partial destination, retry under a distinct name and verify checksum |
| Directory fails to list | More than 1,024 entries, exhausted handles, storage I/O error; split folders, close extra clients and check disk |
| Memory pressure/routing lag | Stop SMB first, preserve web/routing, inspect actual RSS and firmware memory; reduce client activity and avoid simultaneous large workloads |

Read [implementation decision](smb-implementation.md), [security boundaries](../SECURITY.md), and [validation record](smb-validation.md) before distributing or deploying.
