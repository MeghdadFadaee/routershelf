# SMB validation record

Recorded 2026-10-16. SMB remains experimental. Initial implementation validation was local only; a subsequent user-authorized target deployment is recorded below.

## Actual local checks

Development host: macOS 27.0.1, ARM64, Go 1.26.5. Test data and account credentials were synthetic and temporary; no private web settings were needed.

| Check | Observed result |
| --- | --- |
| `make test` | Shell syntax, unchanged public web sample config validation, seven Python localhost stress-runner tests, web regression tests and SMB Go tests pass |
| `go test -race ./cmd/routershelf-smb ./internal/smbwire` | Pass, including startup/root replacement shutdown, authenticated operations, read-only policy, confinement, locks, sharing, concurrent writes and close/delete errors |
| `make build-mipsle build-smb-mipsle` | Both build with CGO disabled, Linux/MIPS little-endian, soft-float; file inspection reports statically linked ELF32 MIPS binaries. SMB executable is approximately 3.6 MiB; this is disk size, not RSS |
| Protocol fuzzing | Ten seconds, two workers of `FuzzMessage`, 554,883 executions; no failure. This is bounded smoke fuzzing, not exhaustive protocol/security validation |
| Native macOS writable smoke script | Built-in `mount_smbfs`: rejects incorrect password, mounts with correct account, creates folder, uploads, moves/renames, downloads with matching SHA-256, deletes files/folder and unmounts successfully |
| Native macOS read-only smoke script | Reads succeed; incorrect password, write, mkdir, rename and delete refused |
| Actual Finder GUI, localhost | Shows selected share and file, Quick Look displays expected text, creates/names a folder, renames a file, copies a folder into the share and back to local storage |
| Finder copy integrity / metadata sample | 2,228,224-byte synthetic file matches SHA-256 after upload and download; a custom extended attribute survives the round trip using an on-disk `._` AppleDouble sidecar |
| Interrupted protocol transfer | 12 MiB written in 64 KiB chunks, connection closed halfway, a new authenticated connection explicitly reopens and writes remaining offsets; on-disk SHA-256 matches. This tests controlled protocol reconnection, not Finder's automatic resume |
| Concurrent operations | Two clients write disjoint 64 KiB ranges to one file, rename through an open handle, close and verify all bytes; conflicting share modes and byte-range locked I/O/truncation fail |
| Confinement | Traversal, encoded traversal, escaping symlinks, rename escape, nonregular data files and writes outside selected root refused; valid internal links remain usable |
| Service independence | SMB config ignores poisoned web domain/root/mount environment values; starts and stops without web service or TLS config. Web tests/build remain independent and its application/supervisor implementation is unchanged |
| Repository hygiene | Public templates have placeholders only; real JSON/hash files, binaries and diagnostic output are excluded from public source distribution; whitespace and local documentation links checked |

The first native attempt used a 16-credit server window: mount/authentication succeeded but mkdir stalled. The final implementation grants up to 128 credits and subsequent normal native mounts passed. The original temporary mount was detached, the recovery server stopped, and temporary files removed. This failure is why native tests accompany protocol tests. Transfer allocations remain separately bounded by the 64 KiB I/O/frame limits and synchronous request handling.

Native command reproduction (development Mac only):

```sh
go build -o dist/routershelf-smb ./cmd/routershelf-smb
python3 -B scripts/test_smb_macos.py "$PWD/dist/routershelf-smb"
python3 -B scripts/test_smb_macos.py "$PWD/dist/routershelf-smb" --read-only
```

The script contacts localhost only and uses a temporary account/share. Its operation worker and mount commands have deadlines. A kernel filesystem operation may still require manual ejection on a client/kernel bug; inspect only the printed/temporary test volume, never unmount a real share to clean up this test. Do not run it on the router.

## Read-only target inspection

SSH inspection confirmed Linux 3.4.113, MIPS, BusyBox 1.24.2 and 126,280 KiB total RAM. The USB filesystem reports exFAT, read/write, `fmask=0000,dmask=0000` and `errors=remount-ro`. Thus mode bits do not isolate private data on that mount; a storage error may change it to read-only. No `smbd`/`smbpasswd` command was found in the inspected PATH, and no TCP 445/1445 listener appeared at that inspection. This is not a complete firmware binary/service inventory.

These read-only observations confirm constraints, not SMB execution compatibility. Private addresses, mount identifiers, credentials and raw router dumps are deliberately absent from this record.

## Initial target checklist (see subsequent deployment results)

This checklist was written before deployment. Completed smoke checks are recorded below; the remaining failure, load and reboot checks are listed at the end. Do not infer target results from compilation or desktop checks:

- Execute the new MIPS binary on Linux 3.4.113 and verify kernel/syscall compatibility, dropped UID/GID and supplementary groups, random source and startup behavior.
- Verify exact USB mount identity, configured selected-root and protected-path checks, exFAT naming/case behavior, disk-full and remount-read-only error reporting, non-root write permissions and metadata sidecar persistence.
- Connect from Finder over the actual LAN and chosen port: registered login, bad password, read-only and writable sharing; folder/file operations, Unicode/case-only renames, packages, tagged/resource-fork files, metadata after restart and application-specific saves. The local custom-attribute sample is not comprehensive Mac metadata validation.
- Transfer large scratch files in both directions with independent checksums; interrupt transfers and service/network connections at bounded points, inspect partial destinations, reconnect and verify safe retry. No automatic native resume or atomic overwrite guarantee is established.
- Coordinate simultaneous client operations with expected locks/share access. Keep other local/FTP/SFTP writers away from the test roots.
- With explicit load bounds and stop criteria, measure resident memory, peak memory, CPU, free space, routing latency and Wi-Fi/SSH health with SMB alone, web alone, and both. Check behavior at connection/handle/directory bounds. A 20 MiB GC target is not a measured memory ceiling.
- Exercise approved USB unavailable/reinsert, supervisor crash/restart, configuration error, binary rollback, independent web/SMB stop/update, host-firewall reload and controlled reboot cases. Do not unplug valuable storage during writes; use backed-up scratch media/data and a deliberate plan.
- Check that allowed private clients connect while guest/WAN/untrusted sources are denied, with no gateway SMB forwarding and no change to web exposure or unrelated rules.

Use [deployment and rollback](smb.md#install-status-update-and-remove). Upload to new names, retain known-good private backups, validate, stop only the verified SMB supervisor, rename into place, restart and verify. Revert the binary/config/script independently if a check fails. Target observations are recorded separately below; smoke checks do not establish unobserved load, reboot, USB-loss or production-readiness guarantees.

## Authorized target deployment and protected root view (2026-10-16)

The user requested read/write access to the USB disk root. Explicit protected
root sharing was added, with separate opt-ins for disk root and public overlap.
Mandatory service directories plus configured firmware metadata are hidden and
inaccessible. Descriptor-pinned parent traversal, no-follow file opens, link
refusal and protected-ancestor rename/delete checks prevent path-only exclusions
from being bypassed. Protocol/race tests cover access, Unicode case variants,
symlink/hardlink aliases, ancestor renames and ordinary cross-directory moves.
Native macOS localhost testing also passed for the protected root profile.

After authorization, the static MIPS binary was staged to a new USB filename;
local/remote SHA-256 matched and target configuration validation passed. It now
runs independently with a separate supervisor/account, UID/GID 65533, no
supplementary groups and a dedicated IPv4 LAN-only firewall chain. Web PID and
configuration were preserved; an HTTPS HEAD check succeeded. Startup/firewall
hooks were extended without executing unrelated startup commands, and the
private configuration/hooks were saved using firmware persistence. The
SMB-only stop/restart command verified process identity and cleanup; native
client access worked after an independent restart.

Observed on the actual device: wrong password refused; native Mac mount;
protected paths absent/unreadable; scratch mkdir, upload, move/rename,
checksum-verified download and deletion. The bounded transfer was 256 KiB,
not a sustained load test. Actual Finder copied a text file to a unique
scratch folder, Quick Look displayed expected content, and the file checksum
matched. Scratch files, sidecars and Mac mounts were cleaned up.

**Observed limitation:** the exFAT mount is owned by another UID and refuses
arbitrary timestamp restoration by the non-root service. Finder copy/open
completed, but the original modification time was not preserved. No silent
metadata-success workaround or privilege increase was introduced. After the
small checks the process used about 6 MiB RSS with a measured high-water mark
near 7 MiB; after restart and a repeat transfer it used about 4.4 MiB. These
snapshots are not a memory ceiling or a routing/load benchmark.

Reboot, USB removal/reinsertion, disk-full/remount-read-only failure, large-file
transfer, native interruption/recovery, complete Mac metadata/application
compatibility and sustained combined resource/load checks remain unperformed.
Private device addresses, disk identifiers, account hashes, passwords, hook
backups and detailed operational instructions remain only in ignored local
outputs and protected router configuration, not in public source examples.

## Refactor deployment regression (2026-10-16)

The refactored static MIPS executable was subsequently deployed with explicit authorization and a retained rollback binary. Installed/upload SHA-256 matched. Native macOS checks again passed for wrong-password refusal, correct login, protected-file exclusions, mkdir, 256 KiB upload, cross-directory move/rename, checksum-verified download, deletion and cleanup. The web process remained unchanged during the SMB update; SMB remained running during the preceding web update. Private configuration, certificates, hooks and firewall policy were preserved.

An already connected Mac initially reused saved authentication during a negative-password mount probe. A temporary localhost TCP relay to the actual LAN service isolated the server identity from that saved connection; the fresh negative login was refused. This avoided disconnecting the user’s existing Finder mount and did not expose real credentials in arguments or logs. It is a native-client protocol/filesystem check, not a new Finder GUI test.

The non-root exFAT timestamp restriction remains. No large native transfer, interruption, USB removal, reboot or sustained combined load check was added during this update.
