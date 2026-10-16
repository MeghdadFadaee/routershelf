# SMB validation record

Recorded 2026-10-16. SMB remains experimental. No SMB executable, configuration or test data was copied to the router, and no router process, firewall, mount, startup hook or routing configuration was changed.

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

## Outstanding checks requiring an approved device session

Do not infer these results from compilation or desktop checks:

- Execute the new MIPS binary on Linux 3.4.113 and verify kernel/syscall compatibility, dropped UID/GID and supplementary groups, random source and startup behavior.
- Verify exact USB mount identity, configured selected-root and protected-path checks, exFAT naming/case behavior, disk-full and remount-read-only error reporting, non-root write permissions and metadata sidecar persistence.
- Connect from Finder over the actual LAN and chosen port: registered login, bad password, read-only and writable sharing; folder/file operations, Unicode/case-only renames, packages, tagged/resource-fork files, metadata after restart and application-specific saves. The local custom-attribute sample is not comprehensive Mac metadata validation.
- Transfer large scratch files in both directions with independent checksums; interrupt transfers and service/network connections at bounded points, inspect partial destinations, reconnect and verify safe retry. No automatic native resume or atomic overwrite guarantee is established.
- Coordinate simultaneous client operations with expected locks/share access. Keep other local/FTP/SFTP writers away from the test roots.
- With explicit load bounds and stop criteria, measure resident memory, peak memory, CPU, free space, routing latency and Wi-Fi/SSH health with SMB alone, web alone, and both. Check behavior at connection/handle/directory bounds. A 20 MiB GC target is not a measured memory ceiling.
- Exercise approved USB unavailable/reinsert, supervisor crash/restart, configuration error, binary rollback, independent web/SMB stop/update, host-firewall reload and controlled reboot cases. Do not unplug valuable storage during writes; use backed-up scratch media/data and a deliberate plan.
- Check that allowed private clients connect while guest/WAN/untrusted sources are denied, with no gateway SMB forwarding and no change to web exposure or unrelated rules.

Use [deployment and rollback](smb.md#install-status-update-and-remove). Upload to new names, retain known-good private backups, validate, stop only the verified SMB supervisor, rename into place, restart and verify. Revert the binary/config/script independently if a check fails. No target load, reboot, kernel compatibility, USB loss recovery or production-readiness claim is made by this change.
