# Maintenance and recovery

## Routine care

| Frequency / event | Check |
| --- | --- |
| After installation, reboot, or power loss | USB mounted at the expected path; accurate time; NTP replies; supervisor and child running; firewall rules reapplied; HTTPS download works |
| Weekly | Disk free space, router memory, repeated errors/restarts, ordinary listing/download, account access |
| Monthly and before expiry | Certificate expiry, renewal errors, DNS/public-IP match, router/toolchain/dependency security updates |
| After changing ISP/public IP | Update DNS A record **and** the two public-IP destination NAT rules; test on mobile data and LAN |
| After changing the disk or USB port | Verify mount ID, update supervisor paths if needed, restore private cache and executable, test restart |
| After adding public files | Check the listing and encoded filenames; ensure that the entire file and its name are intended for publication |
| Before firmware upgrade | Back up private configuration, verify hook compatibility and binary/kernel support, arrange a rollback path |

Do not interpret a successful HTTP request as proof of disk health, NTP synchronization, or renewal. The current implementation does not provide monitoring alerts, filesystem snapshots, or a hard resource quota.

## Read-only health checks

On the router:

```sh
date -u
pidof ntpd
cat /tmp/public-share-supervisor.pid
cat /tmp/public-share-child.pid
cat /tmp/public-share.log
cat /proc/mounts
df -h /media/DISK_ID
iptables -S NB_PUBLIC
iptables -S INPUT
ip6tables -S INPUT
```

Before acting on a PID, inspect `/proc/PID/cmdline` and confirm it belongs to this service. PID files can be stale or reused. Read `/proc/PID/status` for UID/GID and `VmRSS`; do not treat the large virtual address size reported by `ps` as resident RAM use. The observed HTTPS process used around 9 MiB RSS, but traffic and file sizes affect resource use.

Check NTP replies as well as process existence. If `pidof ntpd` is empty, inspect the firmware behavior and start the configured `/tmp/ntpd` command deliberately. The current startup hook starts NTP once; it does not continuously supervise it. The daemon was observed absent during a later check, so reboot and firmware reconnect behavior require special attention. Correct any inaccurate clock before requesting new certificates.

From a client:

```sh
curl -I http://files.example.com/
curl --fail --show-error https://files.example.com/ -o listing.html
curl --fail --show-error https://files.example.com/your-test-file.txt -o probe-download.txt
```

Expected: HTTP redirects to the fixed HTTPS hostname, HTTPS succeeds with normal validation, file bytes match, and navigation works. Also verify that a PUT to a unique nonexistent path returns 405 without creating a file, and a hidden path returns 404. Never use an existing filename for mutation probes.

Check Copy Links by pasting into a local text editor. Check the DPL file contains the expected HTTPS URLs. The browser must allow clipboard access or the fallback may fail; the button shows success/failure. Long listings are truncated at 5,000 entries. The server supports HEAD and byte-range downloads for regular files.

## Certificates and CA trust

The persistent cache contains both the ACME account key and certificate private keys. Leave it intact across restarts and updates. Losing the cache can force new issuance and consume CA rate limits. Automatic renewal needs accurate time, DNS reaching the same public gateway, inbound challenge access, outbound HTTPS to the CA, a writable cache, and a running service. USB removal stops the service and prevents renewal.

Inspect expiry with the `openssl s_client` command in the deployment guide. Renewals run inside `autocert`; there is no separate cron job to edit. The original certificate had roughly a three-month validity, but always inspect the actual expiry instead of assuming a duration. A future automatic renewal has not yet been verified on hardware.

Refresh the CA root bundle periodically from a trusted source. Upload it to a temporary filename and rename it, then restart the child to make a running Go process load the new trust pool. Never solve trust failures by disabling certificate checks.

## Backups

Maintain two separate backup categories:

- **Public source:** this repository, example config, build notes, pinned dependencies, and licensed theme assets. Safe to publish after review.
- **Private operational backup:** actual configuration/hooks, firmware version/settings export, router host-key information, SSH configuration, USB data, and the certificate/account cache. Encrypt it and restrict who can read it.

Copy `/etc/storage` configuration privately and use the firmware's supported settings export. Store the USB backup on another disk, not only alongside the original data. Test restoration to a private location. Exclude all private operational backups from Git and public ZIPs. Avoid backing up a live-changing file halfway through a transfer.

Never place backup archives in `public-storage` unless every byte is intended for public download. exFAT permission masks do not enforce meaningful Unix mode-based protection for the cache; review FTP/SFTP account scope.

## Updating the binary safely

1. Build and test on the computer. Keep the known-good source/toolchain version.
2. Upload a new executable outside the shared root, using a `.new` filename.
3. Check upload checksum/size and executable mode. Preserve a known-good installed binary under a private backup name.
4. Rename the new binary into place. Replacing a pathname by rename leaves the old running process intact until stopped.
5. Verify the child PID's command line, then terminate that child with TERM. The existing supervisor restarts it within its next 10-second check. Expect a brief interruption.
6. Verify log startup, valid HTTPS, downloads, methods, navigation, theme assets, and certificate cache retention. Record the result.

Do not launch a second supervisor. If the supervisor itself needs replacement, stop it, wait until its trap has removed PID files and stopped the child, then start the new script. Starting immediately while the old parent still exits can cause PID detection/cleanup races. Inspect running commands rather than blindly deleting PID files.

The hooks call the installed script path after future startup/firewall rebuilds. Save changed hook/supervisor/env files with `mtd_storage.sh save`; a binary-only update on USB does not require a flash write.

## Rollback

For a binary regression, rename the known-good binary back into place, verify and terminate only the current child, and let the supervisor restart it. Keep the certificate cache. For a script regression, restore the saved script/hooks, reconcile only this service's firewall rules, restart one supervisor, and save flash.

If public access is malfunctioning or unintended, disable the service's TCP 80/443 gateway rules first; keep ordinary Internet NAT and unrelated SSH rules intact. Restore locally and test before re-enabling. Do not re-enable the old HTTP 8888 forward as an automatic rollback from TLS unless plaintext publication is intentionally acceptable.

## Troubleshooting

| Symptom | Likely cause and next check |
| --- | --- |
| Modem panel says no Internet but clients work | Unused WAN port in LAN topology; verify actual default route and client gateway |
| HTTPS connection refused | Child not running, wrong target ports, USB absent, or restart interval; read log and mount status |
| HTTPS timeout from Internet | Incorrect DNS/NAT, forward filter, modem INPUT rule, ISP port filtering, or CGNAT |
| Domain works outside but fails at home | Hairpin DNAT/source-NAT match or LAN routing; test the domain rather than the numeric TLS address |
| Works at home but not on mobile data | LAN success is not external proof; inspect WAN rule counters and actual public-IP ownership |
| Certificate issuance fails | Clock, CA roots, outbound HTTPS, domain whitelist, DNS A/AAAA, challenge reachability, or cache permissions |
| First TLS request fails, next succeeds | Initial issuance exceeded handshake timeout; check for a newly cached certificate before retrying |
| Renewal fails | Service downtime, USB/cache unavailable, inaccurate time, old DNS/NAT address, blocked challenge, or CA rate limit |
| Folder/file missing | Wrong root/mount, dot-prefixed name, outside symlink, nonregular file, read permission, or 5,000-entry truncation |
| FTP login works but upload fails | Account path/write permissions, read-only disk, or passive data ports; perform a real upload/download test |
| Style missing | Failed `/_nais/` asset request, MIME/CSP issue, stale browser cache, or binary built without intended assets |
| Copy button reports failure | Browser clipboard permission/support; test paste manually and try a supported secure browser |
| Service returns after stopping child | Expected supervisor restart; stop the parent to stop the whole service |
| NTP PID missing | Firmware stopped it or wrong process naming; inspect, restart deliberately, and verify replies |

Debug one layer at a time: DNS, gateway rule, forward filter, host firewall, listener, storage, and application. Keep logs private; they can contain addresses and errors identifying the installation.

## Removal without losing shared data

1. Disable/remove only this service's gateway 80/443 rules and scoped hairpin rule. The previously disabled 8888 rule can be removed too. Keep the Internet source-NAT and independent SSH rule unless separately removing them.
2. Verify the supervisor PID and command, terminate it, and wait for its exit trap. Verify that its child stopped and the internal ports are no longer listening.
3. Remove only the two inserted public-share calls/comments from `started_script.sh` and `post_iptables_script.sh`, preserving other firmware/user commands.
4. Delete this service's INPUT jumps for 10080, 10443, and any retained 8888 hook. Remove the dedicated `NB_PUBLIC` chain only after all references are removed. Remove only the matching IPv6 DROP rules for these ports if they belong solely to this installation. Never flush entire tables.
5. Remove the installed supervisor, private environment file, this installation's PID/log files, and private service directory/backups when no longer needed. Preserve `public-storage` and unrelated USB files. Decide separately whether to retain or securely retire the ACME key/cache.
6. NTP is useful to the router beyond file sharing; leave it unless deliberately undoing its configuration. Revert only settings introduced for this deployment if a complete rollback is intended.
7. Save the edited configuration to flash, check the panel/network/SSH still work, and perform a controlled reboot test when convenient.

For removal of the earlier status dashboard, use the separate original guide. That dashboard is already absent on the original router. Removing a service from the device does not remove its teaching source from this repository.

## Environment changes

Use [configuration.md](configuration.md) for configuration precedence and reload steps. The live environment lives at `/etc/storage/routershelf.env`, not in hardcoded source or the old public-access marker. Include this file in encrypted private backups. On config changes, restart the verified supervisor, not just the child, so new values are loaded. Code-only changes still require rebuilding; domain/path/IP changes do not.
