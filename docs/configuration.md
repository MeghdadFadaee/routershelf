# Environment configuration

Network-specific settings live in a private `.env` file. The public `.env.example` contains only illustrative values. Changing a domain, subnet, USB mount, or port does not require editing Go or shell source and does not require rebuilding the application.

## Loading and precedence

The Padavan supervisor loads `/etc/storage/routershelf.env` by default. Override the path with `ROUTERSHELF_ENV_FILE=/absolute/path/to/config`. It exports the settings to the child application and supplies matching command-line values. Run `--check-config` to validate without starting NTP, changing firewall, or launching a process.

The supervisor **sources** the file as trusted shell configuration. Keep it administrator-owned, restrict its mode, and use simple `RS_KEY=value` assignments. A dotenv file from an untrusted person can execute commands when sourced. This format is not an arbitrary web input or a general dotenv parser; shell quoting rules apply.

The Go binary reads exported `RS_*` values; it does not automatically read or execute `.env`. Its flags take precedence over environment defaults. For local use:

```sh
set -a
. ./.env
set +a
go run ./cmd/routershelf
```

With no configured IP, the standalone binary defaults to loopback. `RS_DOMAIN` enables HTTPS; without it, the standalone binary can serve plain HTTP for development. The deployment supervisor requires a domain and distinct unprivileged internal ports.

## Settings

| Variables | Purpose |
| --- | --- |
| `RS_DOMAIN` | Single HTTPS certificate hostname, without scheme or port |
| `RS_LAN_IP`, `RS_LAN_SUBNET`, `RS_LAN_INTERFACE` | Bind address and modem firewall scope |
| `RS_USB_MOUNT` | Exact expected USB mount point |
| `RS_PUBLIC_SUBDIR`, `RS_SERVICE_SUBDIR` | Subdirectory names used to derive share/service paths |
| `RS_ROOT`, `RS_BINARY`, `RS_CERT_CACHE`, `RS_CA_BUNDLE` | Optional absolute path overrides |
| `RS_HTTP_PORT`, `RS_HTTPS_PORT` | Internal listener/firewall ports; default 10080 and 10443 |
| `RS_PUBLIC_ACCESS` | `false`: LAN-only sources; `true`: also allow forwarded public sources |
| `RS_DROP_PRIVILEGES` | Run application as UID/GID 65534; normally `true` |
| `RS_MEMORY_LIMIT`, `RS_CPU_LIMIT` | Go GC memory target and GOMAXPROCS |
| `RS_CHECK_INTERVAL`, `RS_MIN_YEAR` | Supervisor poll interval and coarse reset-clock guard |
| `RS_NTP_SERVER1`, `RS_NTP_SERVER2` | NTP startup peers |
| `RS_SUPERVISOR_PATH`, `RS_RUNTIME_DIR` | Installed script and runtime PID/log directory |
| `RS_GATEWAY_*` | Public/LAN gateway addresses and interfaces, used by the NAT plan |
| `RS_PUBLIC_HTTP_PORT`, `RS_PUBLIC_HTTPS_PORT` | Public standard ports: 80 and 443 |
| `RS_PUBLIC_SSH_PORT`, `RS_SSH_TARGET_PORT` | Independent SSH mapping in the NAT plan |

The standard public 80/443 configuration is intended for ACME validation and normal redirects. Changing public HTTPS away from 443 requires redirect handling changes too; the current redirect targets the standard HTTPS hostname. These plan values do not change application redirects or DNS automatically.

Do not place the binary, trust bundle, or certificate cache inside the public root. Deployment paths are absolute and may not contain spaces or `..` in the supervisor. Private keys stay on USB outside the root; a filesystem/account policy must protect them from broad FTP/SFTP access.

## Installing your configuration

```sh
cp .env.example .env
# Edit values in .env; set RS_PUBLIC_ACCESS=true only if public sharing is intended.
ROUTERSHELF_ENV_FILE="$PWD/.env" sh deploy/padavan/supervisor.example.sh --check-config
make nat-plan
ssh router 'cat > /etc/storage/routershelf.env; chmod 600 /etc/storage/routershelf.env' < .env
```

Install the matching supervisor from `deploy/padavan/supervisor.example.sh`, validate it on the target, and keep the startup/firewall hooks pointing to its installed path. `mtd_storage.sh save` persists the private environment alongside the hooks on the observed firmware. Back it up privately; it is not a public example.

The obsolete `/etc/storage/public-share-public` marker is no longer used. `RS_PUBLIC_ACCESS` is the source of truth. Keeping an old marker does not grant access with this version.

## Applying changes

Editing the file alone does not reload a running supervisor or application's environment. Stop the verified supervisor, wait for its child and PID cleanup, then start one supervisor again. Save the edited env to flash. This can briefly interrupt downloads.

For domain/IP/port changes, review the printed gateway plan and update DNS/NAT manually as needed. For shared root or USB changes, verify mount and read permissions. Keep the existing certificate cache when the domain is unchanged. Rebuilding is required only for code/theme changes, not configuration changes.

The firewall reconciles its own IPv4 rules, including previous internal ports, while blocking traffic during replacement. IPv6 blocks on old ports are retained; review them during cleanup. A failure can leave temporary IPv4 DROP rules to keep the service closed. Inspect and resolve the failure before removing these blocks. Do not run simultaneous firewall managers.

## Git and archives

`.env` and private variants are ignored; `.env.example` is explicitly included. Public ZIPs must include only the example. `.gitignore` does not remove already committed secrets: check staged content before publication and rotate any exposed credentials.
