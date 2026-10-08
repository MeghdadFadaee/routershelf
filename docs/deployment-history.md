# Deployment history and verification

This is a sanitized account of the work completed on the original home network. Example addresses are used throughout. It records the final behavior and the limits of validation, rather than implying that every suggested test was performed.

## 1. Network inspection and correction

The LHG5 was the Internet gateway. The NW-651D supplied Wi-Fi and additional LAN ports, with the gateway connected to a **LAN** port. Internet already worked on the user's computer. The empty modem WAN port explained the panel's disconnected-WAN indication.

We installed and saved the storage host's default route through the LHG5 on `br0`, metric 1. DHCP remained enabled on the NW-651D; clients were configured to use the LHG5 as their gateway and public DNS servers. Existing static leases and ARP bindings were retained. Bounded Internet and DNS-related reachability checks succeeded. Moving the cable to WAN was not necessary for this topology.

## 2. SSH discovery and temporary status dashboard

Authorized SSH access revealed BusyBox, OpenSSH/SFTP, vsftpd, curl, cron, and iperf3. A shell/Netcat dashboard was built on port 8080 to show uptime, approximate RAM use, load, and bounded connectivity probes. LAN firewall rules, IPv6 blocking, startup/firewall hooks, and flash saving were added. HTTP responses and browser rendering were checked.

At the user's request, this dashboard was completely removed from the router: scripts, hooks, dedicated firewall rules, runtime files, and installation backups were cleaned up, and flash was saved. The public documentation project and dashboard teaching templates were retained on the computer.

## 3. USB storage and transfer services

A roughly 1 TB USB disk was mounted read/write as exFAT. A temporary local file was written, read back, and removed successfully. FTP configuration allowed authenticated local users and writes, disabled anonymous access, and selected passive ports 50000–50100. An anonymous FTP attempt returned 530.

**An authenticated FTP upload/download was not completed**, because credentials were not provided for that test. Configured write support and successful local disk I/O do not prove authenticated FTP permissions. SFTP login through the existing SSH key was verified; an SFTP file-transfer test was not completed at that stage.

## 4. Gateway port forwarding cleanup

A WAN TCP 222 rule was created to forward SSH to the NW-651D's TCP 22. Eight old destination-NAT forwarding rules were removed at the user's request. The Internet source-NAT rule was preserved. This SSH forwarding rule remains separate from file sharing.

## 5. Public USB directory on port 8888

A Go directory server was compiled for Linux/MIPS little-endian and deployed from a hidden service directory outside `public-storage`. It listed folders and forced regular-file downloads. GET/HEAD were allowed; upload and deletion methods were rejected. Hidden paths, traversal, and escaping symlinks were blocked. A child supervisor, USB mount checks, bounded connections, firewall hooks, and flash persistence were installed.

A WAN TCP 8888 forward was created after explicit approval for unauthenticated public access. Listing and download of a small test file succeeded locally; its checksum matched. PUT returned 405 without creating a file; traversal returned 404. Initial resident memory was approximately 3 MiB. External download was not independently verified at this stage.

## 6. HTTPS and standard ports

DNS was confirmed to point to the LHG5's public IPv4 address. The router clock was corrected. NTP server settings and a BusyBox NTP startup command were added. A current CA root bundle was installed outside the public directory so the Go ACME client could validate its outbound TLS connection.

The Go server gained `autocert`, with a whitelist for one domain and a persistent certificate/account cache on USB. It listens internally on TCP 10443 for HTTPS and TCP 10080 for ACME HTTP challenges and redirects. A trusted Let's Encrypt certificate was issued. Automatic renewal support is enabled; a future renewal has not yet been observed.

The gateway forwards public TCP 443 to internal 10443 and TCP 80 to internal 10080. The rules match the deployment's public IPv4 address. A scoped LAN source-NAT rule enables hairpin access so the same domain works from inside the home. The old WAN 8888 rule was disabled; no server listens on 8888.

Validated: certificate hostname/issuer/expiry, trusted HTTPS without disabling validation, HTTP 308 redirect, listing, file download and byte equality, PUT 405, hidden-path 404, and browser rendering at the real domain. Certificate authority validation demonstrated inbound challenge reachability. A separate mobile-data or remote-client download remains an additional verification step. Resident memory after TLS was approximately 9 MiB. The server runs as UID/GID 65534 after dropping privileges.

NTP replies were observed during setup. A later documentation-time check did not return an NTP PID, although the clock remained correct. Do not assume that NTP remains running: verify it after firmware events and reboot, as described in the operations guide.

## 7. NAIS styling

NAIS CSS, JavaScript, and favicon assets were copied from the user's local clone and embedded in the Go binary. No nginx installation was needed. The HTML uses an autoindex-like heading and preformatted file rows. Theme assets are served from a reserved `/_nais/` namespace.

Small integration changes: fixed row alignment, mobile header wrapping, a read-only footer, explicit file markers so extensionless files enter copied links/playlists, and copy success/failure feedback. The original NAIS checkout was not edited.

The dark theme rendered in the browser. HTTPS file download stayed intact. A downloaded DPL playlist contained the correct HTTPS file URL. The Copy Links button reported `Copied!`; the automation clipboard readback did not independently confirm the OS clipboard content, so manually paste the result when checking a new browser.

## Current service inventory

| Component | State |
| --- | --- |
| LAN status dashboard, TCP 8080 | Removed from router; teaching source retained |
| FTP | Configured for authenticated read/write; end-to-end authenticated transfer still requires verification |
| SFTP/SSH | Key login checked; WAN TCP 222 forward retained |
| Public HTTP, TCP 8888 | Old gateway rule disabled; listener replaced |
| HTTP, public TCP 80 | ACME challenge responses and redirect to HTTPS |
| HTTPS, public TCP 443 | Read-only directory server with NAIS |
| Certificate renewal | Enabled in code; future renewal not yet exercised |
| Boot persistence | Hooks and flash save checked; full reboot test outstanding |

Future checks should be recorded with dates and outcomes. Avoid promoting a configured feature to a tested guarantee.

## 8. Environment-based configuration

Network and installation values were moved into an ignored private `.env` on the computer and `/etc/storage/routershelf.env` on the router. A tracked `.env.example` supplies public placeholders. The same supervisor source now loads the configuration; the Go binary reads exported settings with CLI overrides.

The live router was migrated with the same domain, addresses, paths, and listener ports. The existing certificate cache was retained and a pre-migration script/binary backup kept privately. The old public-access marker no longer controls permissions; `RS_PUBLIC_ACCESS` does. Owned IPv4 firewall rules were rebuilt from the configuration, removing the unused 8888 jump.

Validated: example and live config checks, invalid-port rejection, Go environment/override and unsafe-config tests, MIPS build, live trusted HTTPS/download, HTTP redirect, PUT 405, private env path 404, live chain scope, dropped UID/GID, and flash save. The full reboot and future renewal checks remain outstanding. The NAT-plan utility prints configuration guidance without changing the gateway.

## 9. Static pages in the same shared root

The application now renders a directory's readable regular `index.html` before listing it, and serves direct HTML requests inline. Recognized CSS/JavaScript/image/font assets have explicit MIME types. Other files retain attachment downloads. No separate root or URL prefix, new env settings, or gateway rules were added.

The binary was deployed with the previous version kept for rollback. Tests covered folder/root index precedence, direct HTML and assets, HEAD, preserved downloads, listing link attributes, and an escaping index symlink. A temporary live page rendered over HTTPS with CSS and JavaScript; its direct HTML link rendered too. HEAD confirmed HTML without attachment disposition, a normal text file still had attachment disposition, and PUT remained 405. Temporary probe files were removed after testing. Static pages intentionally permit browser-side active content; no PHP/CGI/server-side execution was added.
