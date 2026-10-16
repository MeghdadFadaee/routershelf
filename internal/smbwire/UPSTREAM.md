# Embedded SMB implementation

Imported from [softwarity/smbserver v0.1.1](https://github.com/softwarity/smbserver/tree/v0.1.1), commit `88586e668400859c882428b308408c26086bb1d5` (2026-10-04). Go module checksum: `h1:TV8VpKrzyiaZGUl9A6+ES4rdOEY4NMRp+xo9LYojXxY=`. Apache-2.0 license retained in `LICENSE`. All upstream top-level Go source, unit, socket and fuzz tests were imported; its command, CI and native-client integration scripts were not imported. Package name remains `smbserver`.

This is a maintained local adaptation, not an unmodified dependency. RouterShelf changes:

- Four connections, two sessions/connection, two trees/session, 64 open handles/server, eight compound commands, 128 credits and 64 KiB I/O. Directory scans fail explicitly above 1,024 entries; byte-range locks are bounded to 128/file. Five-minute authenticated read deadline.
- `DiskMetadata` rejects named streams and removes their capability advertisement; permits `.DS_Store` and disk AppleDouble sidecars. RouterShelf always enables it. Upstream's bounded ephemeral streams remain for upstream regression tests, inaccessible through the shipped command.
- Filesystem case sensitivity is explicitly configurable for exFAT. Failed filesystem size queries return an error rather than synthetic capacity.
- Bidirectional share access checks before opening/truncating, byte-range lock enforcement on data I/O and truncation, write-through and close synchronization, close/delete error reporting, and access checks on attribute changes. Nonblocking file opens verify the actual regular-file identity before truncating to avoid blocking on a replaced FIFO. Unsupported ACL and per-file protection-mode changes return an error in the RouterShelf profile.
- Protocol trace via environment disabled, preventing accidental filenames in logs. Optional explicit diagnostics report numeric commands/status/credits and peer events without filenames or authentication tokens. Upstream authentication test isolated into subtests for the smaller connection bound.
- Added RouterShelf policy, metadata, sharing, transfer integrity and reconnect regression tests. Startup can require an expected root identity and signals ready only after confinement is initialized.

Review upstream changes and security issues before updating. Reapply/review each adaptation and run `make test`, race checks, fuzz checks and both builds. Require native Finder and target validation before promoting support claims. This young implementation has not received a RouterShelf independent security audit; it is for private trusted clients, never Internet exposure. Its internal API is not a supported public RouterShelf library.

The initial 16-credit adaptation stalled the native macOS client before mkdir; 128 credits passed the local native smoke check. Credits bound the request window, while the synchronous parser and frame/I/O limits independently bound allocations. Preserve this native-client regression check when tuning credits.

RouterShelf also adds `guarded_root.go` and its tests for explicit disk-root views: protected relative subtrees are hidden/blocked, aliases refused, protected ancestors cannot be renamed/deleted, and parent descriptors are pinned with os.Root. Cross-directory rename and descriptor timestamp operations use BSD-licensed golang.org/x/sys/unix. Default subdirectory shares preserve the original os.Root behavior.
