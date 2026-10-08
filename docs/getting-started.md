# Deploy the public HTTPS directory service

## Architecture and paths

```text
Internet or LAN browser -> files.example.com
    public TCP 80  -> gateway DNAT -> storage host TCP 10080 (ACME/redirect)
    public TCP 443 -> gateway DNAT -> storage host TCP 10443 (HTTPS listing/download)
```

The service shares only `/media/DISK_ID/public-storage`. Installation layout:

```text
/media/DISK_ID/public-storage/                         Published files
/media/DISK_ID/.public-share-service/public-directory  Executable
/media/DISK_ID/.public-share-service/ca-certificates.pem
/media/DISK_ID/.public-share-service/certificates/      Private ACME account/key cache
/etc/storage/public-share-service.sh                   Supervisor/firewall script
/etc/storage/routershelf.env                           Private configuration
/tmp/public-share-supervisor.pid                       Runtime parent PID
/tmp/public-share-child.pid                            Runtime child PID
/tmp/public-share.log                                  Runtime log
```

The certificates directory is private administrative material, despite its location on the same disk. Never put it inside `public-storage`, include it in public backups, or symlink it into the shared root. The server reserves `/_nais/` for embedded theme assets; use a different name for a real directory that must be shared.

## 1. Prepare source and example configuration

Build on a computer with Go, not on the router. The tested build used Go 1.26.5 and pinned dependencies in `go.mod`/`go.sum`. Confirm the current toolchain's compatibility with the target's old kernel before upgrading. The program uses `os.OpenRoot` to enforce the shared-directory boundary.

```sh
mkdir -p dist
go test ./...
GOOS=linux GOARCH=mipsle GOMIPS=softfloat CGO_ENABLED=0 \
  go build -trimpath -ldflags='-s -w' -o dist/routershelf-linux-mipsle ./cmd/routershelf
```

Use `mipsle` only for the matching target CPU. The binary has no PHP, nginx, or dynamically linked runtime requirement. Preserve dependency checksums. Rebuild and test for other devices rather than downloading an arbitrary executable.

Copy `.env.example` to `.env` and set the actual domain, USB mount, LAN IP/subnet/interface, and internal ports there. Set `RS_PUBLIC_ACCESS=true` only for intentional public sharing. The supervisor and Go binary derive paths from these settings; there is no per-network source edit or rebuild. See [configuration](configuration.md).

```sh
cp .env.example .env
ROUTERSHELF_ENV_FILE="$PWD/.env" sh deploy/padavan/supervisor.example.sh --check-config
make nat-plan
```

The supervisor owns `NB_PUBLIC`. It reconciles this chain and its IPv4 INPUT jumps when settings change, temporarily blocking the affected ports while rebuilding. If a firewall operation fails, temporary blocks may remain; inspect rather than bypass them. Old IPv6 DROP rules are retained for safety and can be reviewed separately. The former 8888 IPv4 jump is removed during reconciliation.

## 2. Prepare clock and outbound trust

A correct clock is required for TLS and certificate operations. Configure the firmware's NTP servers, for example `time.cloudflare.com` and `pool.ntp.org`, and check actual replies. If the clock is badly wrong, set it once from an accurate trusted computer, then verify NTP synchronization.

The original supervisor creates `/tmp/ntpd` as a symlink to `/bin/busybox` and launches that path. This lets `pidof ntpd` find the process; starting `busybox ntpd` can leave the process named `busybox` and cause duplicate launches. Firmware events can still stop NTP, so monitor it separately. The service supervisor does not continuously restart NTP.

Install a trusted CA bundle from an authenticated source. The original used the [curl CA bundle](https://curl.se/docs/caextract.html), with certificate verification enabled for its download:

```sh
curl --fail --show-error --silent https://curl.se/ca/cacert.pem -o ca-certificates.pem
ssh router 'mkdir -p /media/DISK_ID/.public-share-service'
ssh router 'cat > /media/DISK_ID/.public-share-service/ca-certificates.pem' < ca-certificates.pem
```

The supervisor passes its path as `SSL_CERT_FILE`. This fixes outbound trust; it is different from the server certificate issued for your domain. Do not bypass TLS validation with `-k` or insecure Go settings.

## 3. Install and start

Upload to a new filename first:

```sh
ssh router 'cat > /media/DISK_ID/.public-share-service/public-directory.new; chmod 755 /media/DISK_ID/.public-share-service/public-directory.new' < dist/routershelf-linux-mipsle
ssh router 'mv /media/DISK_ID/.public-share-service/public-directory.new /media/DISK_ID/.public-share-service/public-directory'
ssh router 'cat > /etc/storage/routershelf.env; chmod 600 /etc/storage/routershelf.env' < .env
ssh router 'cat > /etc/storage/public-share-service.sh; chmod 755 /etc/storage/public-share-service.sh' < deploy/padavan/supervisor.example.sh
ssh router '/etc/storage/public-share-service.sh --check-config'
```

Do not overwrite a running binary in place; see the update procedure in the operations guide. These commands assume the intended shared directory already exists and that the paths match the mounted disk.

The supervisor checks for the exact mounted USB path every 10 seconds, stops the child when the mount disappears, and restarts it when available. It sets `GOMEMLIMIT=32MiB` and `GOMAXPROCS=2`; the memory limit is a Go garbage-collection target, **not** a hard process limit. The program then drops supplementary groups and runs as UID/GID 65534. Directory traversal permissions must allow that user to read the shared data and use its cache.

Enable public mode only when publishing all current and future shared-directory content is intended:

```sh
# Set RS_PUBLIC_ACCESS=true in the private env for intentional public access.
ssh router '/etc/storage/public-share-service.sh'
```

The modem firewall uses `NB_PUBLIC` on INPUT for the internal service ports. Public mode accepts packets arriving on `br0` destined for the host address, including forwarded packets with Internet source addresses. A LAN-source-only allow rule would reject those DNAT connections. The final chain rule drops other traffic. IPv6 access is blocked; no IPv6 publication was implemented.

## 4. Save firmware hooks

Append each command **once**, retaining the existing script contents:

```sh
# /etc/storage/started_script.sh, after network startup
/etc/storage/public-share-service.sh

# /etc/storage/post_iptables_script.sh, after firewall rebuild
/etc/storage/public-share-service.sh --firewall
```

Back up these scripts privately before editing. The firmware's `/etc/storage` is a RAM-backed working directory. Persist changes with its supported helper:

```sh
mtd_storage.sh save
```

Use this command on the matching firmware only. It saves the small configuration hooks to flash; the binary, CA bundle, and ACME cache remain on USB. Avoid saving to flash for routine traffic, logs, or certificate renewal. A successful save is not a reboot test.

## 5. Configure DNS and gateway NAT

Point the public DNS A record for `files.example.com` to the gateway's public IPv4 address. Do not advertise AAAA unless you deliberately implement and test IPv6. Check `dig +short files.example.com A` and `dig +short files.example.com AAAA`.

Create the following RouterOS NAT rules through WebFig or an authorized RouterOS administrator session. Preserve the existing Internet source-NAT rule and separate SSH forward.

| Purpose | Chain and match | Action |
| --- | --- | --- |
| HTTPS | `dstnat`, destination `203.0.113.10`, TCP destination port 443 | `dst-nat` to `192.168.50.1:10443` |
| HTTP/ACME | `dstnat`, destination `203.0.113.10`, TCP destination port 80 | `dst-nat` to `192.168.50.1:10080` |
| LAN hairpin reply | `srcnat`, source `192.168.50.0/24`, destination `192.168.50.1`, TCP destination ports `10080,10443`, output `ether1` | `src-nat` to `192.168.50.2` |
| Separate SSH | `dstnat`, input `pppoe-out1`, TCP destination port 222 | `dst-nat` to `192.168.50.1:22` |

The HTTPS/HTTP rules deliberately match the public address without a WAN-only input restriction, so LAN clients using the same public DNS answer can hit them too. Hairpin source NAT routes replies back through the gateway rather than directly to the LAN client. Restrict it to these service ports. Review filter rules as well: NAT alone does not authorize a packet that a forward-chain filter drops.

If the public IP changes, update both DNS and these two destination-address matches. Dynamic DNS alone does not update the hardcoded NAT rules. A private ISP address or CGNAT requires a different connectivity solution; port forwarding on the home router cannot bypass an upstream NAT you do not control.

## 6. Obtain and verify HTTPS

`autocert.Manager` whitelists the configured domain, accepts the CA terms in code, obtains a certificate on demand, stores its account/certificate cache on USB, and schedules renewal while running. Review the CA terms before enabling this configuration. Public certificates are recorded in certificate transparency logs.

The HTTP handler serves ACME challenges and redirects ordinary GET/HEAD requests to the fixed HTTPS domain with HTTP 308. Unknown HTTP hosts are rejected; redirects do not trust arbitrary Host headers. TLS uses a minimum of TLS 1.2. The application limits connections and header sizes and uses request timeouts.

Issue the first request at `https://files.example.com/`. Initial issuance may exceed the short handshake timeout and fail once; inspect logs and the cache, then retry after issuance completes. Do not repeatedly delete the cache or loop certificate requests, as CA rate limits apply.

```sh
curl --fail --show-error https://files.example.com/
curl -I http://files.example.com/
curl --fail --show-error https://files.example.com/your-test-file.txt -o downloaded-test.txt
openssl s_client -connect files.example.com:443 -servername files.example.com </dev/null 2>/dev/null | openssl x509 -noout -subject -issuer -dates
```

Compare the download with its source. Use normal trusted certificate validation. Test once from outside the LAN, such as mobile data with Wi-Fi disabled. A LAN success verifies hairpin behavior; a successful CA challenge verifies a challenge route, not every external client's full download path.

## 7. How NAIS is integrated

The Go executable embeds the theme files under `internal/theme/nais`: `autoindex.css`, `autoindex.js`, and favicons. The listing references them under `/_nais/`. This reproduces the theme without nginx's `sub_filter` or its installer. Folder links remain navigable; regular files carry `data-file="true"` and download attributes. The script uses that marker for copied links and DPL playlists, including extensionless files.

The bundled integration adjusts row width and the mobile heading, changes the footer to read-only storage, and adds copy feedback/fallback handling. The HTML template escapes names. Assets have explicit MIME types; HTML and recognized CSS/JavaScript/image/font assets are served inline with explicit MIME types; other files use `application/octet-stream` and attachment disposition. A directory with a readable regular `index.html` displays that page instead of its listing. PHP/CGI are not executed. The listing retains its strict CSP; static pages permit inline scripts/styles and external assets, with object embedding and framing blocked.

To update the style, review upstream changes, copy the intended assets to this source tree, retain the NAIS license, preserve the integration changes, rebuild, and deploy using the rollback procedure. Do not publish the upstream landing page as the directory listing.

References: [Go autocert](https://pkg.go.dev/golang.org/x/crypto/acme/autocert), [Let's Encrypt challenges](https://letsencrypt.org/docs/challenge-types/), [NAIS](https://github.com/meghdadfadaee/nais).
