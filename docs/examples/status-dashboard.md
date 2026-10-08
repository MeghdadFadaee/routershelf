# Build a Small LAN Web Service on a Linux Router

> Historical example: this LAN dashboard was removed from the original router. For the current public HTTPS/NAIS service, read [getting-started.md](../getting-started.md) and [operations.md](../operations.md). The two services have different access and security models.

This guide explains how to build, install, and maintain a service like the included router dashboard. It assumes you own or administer the router and have an authorized SSH account. The example is deliberately small enough to run without installing a language runtime.

## 1. Understand the architecture

```text
LAN browser -> router TCP port 8080 -> Netcat -> shell request handler
                                               |
                                               +-> /proc status files
                                               +-> bounded ICMP probes
                                               +-> HTTP headers + HTML response
```

The router's management panel remains on its existing port. The new service does not change DHCP, routing, Wi-Fi settings, credentials, or the panel. Being in a directory opened by PhpStorm does not mean the router runs PHP.

The browser requests `/` or `/status`. The handler reads a fixed request path, gathers metrics, sends a response, and exits. Netcat waits for the next client. With no browser open, no periodic probe runs.

## 2. Inspect your target before designing the service

Use your own SSH alias. In the examples, `router` is an illustrative alias, not a credential bundled with this repository.

```sh
ssh router
uname -a
busybox
nc --help
cat /proc/meminfo
cat /proc/mtd
mount
df -h
netstat -lnt
iptables -S INPUT
ip6tables -S INPUT
```

Check these requirements:

- Linux exposes `/proc/uptime`, `/proc/meminfo`, and `/proc/loadavg`.
- The installed shell supports `read -r -t`; this is an extension, not portable POSIX `sh` behavior.
- Netcat supports persistent `-ll` listening with `-e PROGRAM`. Other Netcat implementations have different options.
- `awk`, `ping -c ... -W ...`, `nohup`, `grep`, `iptables`, and `ip6tables` exist.
- The chosen TCP port is unused.
- You know the LAN bridge interface, LAN address, client subnet, and upstream gateway.
- The firmware provides a writable configuration directory and an understood mechanism to save it across power cycles.

The original router had about 128 MiB RAM, a read-only SquashFS root, and a 512 KiB persistent Storage partition. Its runtime `/etc/storage` directory was backed by RAM. `df` alone does not reveal the persistent flash capacity: inspect `/proc/mtd` too. Do not fill flash or assume that a writable runtime file survives reboot.

Read the device's own help first. BusyBox builds can omit applets or features even when the same version is reported. If a suitable HTTP daemon is already installed, consider using it instead of Netcat; do not repurpose the management panel daemon without documentation.

## 3. Write a fixed, read-only request handler

See [`../../examples/status-dashboard/scripts/status-response.sh`](../../examples/status-dashboard/scripts/status-response.sh).

The script accepts only `GET /` and `GET /status`, returning 404 for other requests. It consumes the request headers before sending the page. Request text is never executed, passed to `eval`, or used as a filesystem path.

A minimal HTTP response needs a status line, headers, a blank line, and a body. Lines in the header section use CRLF:

```sh
printf 'HTTP/1.1 200 OK\r\nContent-Type: text/html; charset=utf-8\r\nConnection: close\r\n\r\n'
```

The example does not send Content-Length for successful responses. Instead, it sends `Connection: close`; completion of the handler closes the response stream. Its empty 404 response includes `Content-Length: 0`.

The included headers disable browser caching, prevent MIME sniffing, block framing, and permit only the inline CSS used by the page. There are no cookies, external fonts, scripts, or analytics.

### Metrics and interpretation

| Metric | Source | Interpretation |
| --- | --- | --- |
| Uptime | `/proc/uptime` | Converted to days, hours, and minutes |
| Memory | `/proc/meminfo` | Total minus free, buffers, and cached memory; approximate, not MemAvailable |
| Load averages | `/proc/loadavg` | Workload averages over 1, 5, and 15 minutes; not CPU percentages |
| Gateway reachability | `ping -c 1 -W 1` | One bounded ICMP probe |
| Internet probe | `ping -c 1 -W 2` | One bounded probe to a chosen public IP |

A failed probe can mean blocked ICMP rather than an outage. The IP probe does not test DNS, HTTPS, or every website. Successful Internet access from the router also does not prove that every client has the correct DHCP lease.

The page refreshes through `<meta http-equiv="refresh" content="30">`. Every open tab generates its own requests. This is a live snapshot, not historical monitoring.

## 4. Prepare configuration on your computer

From the repository root, enter the standalone example directory first:

```sh
cd examples/status-dashboard
```

```sh
cp config.example.sh config.sh
```

Edit `config.sh` to match your router. The example subnet is illustrative:

```sh
LAN_INTERFACE=br0
LAN_IP=192.168.50.1
LAN_SUBNET=192.168.50.0/24
GATEWAY_IP=192.168.50.2
PROBE_IP=1.1.1.1
PORT=8080
```

Use numeric IP addresses, a valid CIDR subnet, and an unused unprivileged TCP port. The configuration is sourced as shell code and must be treated as administrator-controlled code. Do not generate it from HTTP input. Do not commit the real configuration.

Before upload, check syntax:

```sh
sh -n scripts/status-response.sh
sh -n scripts/status-service.sh
sh -n config.sh
```

Syntax checks on a desktop do not prove support for BusyBox extensions. Repeat them on the router, then exercise the requests there.

## 5. Upload the files

The following commands assume the reviewed Padavan-style `/etc/storage` layout. Adapt them for other firmware; do not invent persistence paths.

```sh
ssh router 'mkdir -p /etc/storage/router-status; chmod 700 /etc/storage/router-status'
ssh router 'cat > /etc/storage/router-status/config.sh' < config.sh
ssh router 'cat > /etc/storage/router-status/status-response.sh' < scripts/status-response.sh
ssh router 'cat > /etc/storage/router-status/status-service.sh' < scripts/status-service.sh
ssh router 'chmod 600 /etc/storage/router-status/config.sh; chmod 700 /etc/storage/router-status/status-response.sh /etc/storage/router-status/status-service.sh'
```

Using SSH input redirection avoids depending on an SFTP subsystem. Do not replace unrelated files or upload an entire router backup into the repository.

Check syntax and file ownership on the router:

```sh
ssh router 'sh -n /etc/storage/router-status/status-response.sh; sh -n /etc/storage/router-status/status-service.sh; ls -l /etc/storage/router-status'
```

## 6. Restrict access before starting the listener

See [`../../examples/status-dashboard/scripts/status-service.sh`](../../examples/status-dashboard/scripts/status-service.sh). The service manager applies its firewall rules before starting Netcat. It uses a dedicated IPv4 chain, `RS_STATUS`, reached from INPUT for the configured port.

The only allow rule requires all three:

1. Arrival on the configured LAN interface.
2. Source address in the configured client subnet.
3. Destination address equal to the configured router LAN IP.

All other IPv4 traffic to the service is dropped. IPv6 traffic to its port is also dropped. The stock Netcat listener may bind all addresses, including an IPv6 wildcard socket; firewall enforcement is therefore essential.

Run and inspect the rules first:

```sh
ssh router '/etc/storage/router-status/status-service.sh --firewall'
ssh router 'iptables -S INPUT; iptables -S RS_STATUS; ip6tables -S INPUT'
```

Do not continue if commands fail or the expected rules are missing. Some embedded firewall builds behave unexpectedly with `iptables -C`; this example checks listed rules instead. It requires `iptables -S` support.

The example does not automatically change an existing chain to new LAN settings. If configuration changes, stop the listener, remove the service's old rules using its old configuration, then apply the new settings. Do not flush the router's overall firewall.

There is no authentication. Any permitted LAN client can read the dashboard. See [the security model](../../SECURITY.md) for limitations. Keep it off public and untrusted guest networks.

## 7. Start and verify the service

```sh
ssh router '/etc/storage/router-status/status-service.sh'
```

The manager launches:

```sh
nohup nc -ll -p "$PORT" -e "$SERVICE_DIR/status-response.sh" \
  </dev/null >/tmp/router-status.log 2>&1 &
```

The PID goes into `/tmp/router-status.pid`. Logs and PID files are temporary; the executable scripts and configuration belong in persistent storage.

From a permitted LAN client, substitute your router's address and configured port:

```sh
curl --max-time 10 -i http://192.168.50.1:8080/
curl --max-time 10 -i http://192.168.50.1:8080/status
curl --max-time 10 -i http://192.168.50.1:8080/missing
```

Expect 200 and an HTML page for the first two, and 404 for the third. Open the page in a browser and check layout, metrics, and refresh behavior. If available, try from a guest/VLAN client outside the allow rule; it should not connect. Verify IPv6 is blocked too.

A request from the router itself may fail because it arrives on loopback rather than the permitted LAN interface. Test from a LAN client; do not weaken the access rule just to make a loopback test succeed.

To diagnose startup:

```sh
ssh router 'cat /tmp/router-status.log; netstat -lnt; cat /tmp/router-status.pid'
```

A running PID is not sufficient evidence that the listener is healthy. Check its command line and make an actual HTTP request.

## 8. Configure persistence without overwriting existing hooks

On the reviewed firmware, `/etc/storage/started_script.sh` runs after startup when networking is ready. `/etc/storage/post_iptables_script.sh` runs after firmware firewall rebuilds. Inspect both files before editing them. Back them up privately on the router or your own computer.

Add these lines once to `started_script.sh`, before any unconditional exit:

```sh
# Router status service
/etc/storage/router-status/status-service.sh
```

Add these lines once to `post_iptables_script.sh`, also before any unconditional exit:

```sh
# Router status service firewall
/etc/storage/router-status/status-service.sh --firewall
```

Preserve all existing custom behavior. Do not replace the whole hook or append duplicates during repeated installation. Keep hook permissions unchanged.

On this firmware, save the edited storage files with:

```sh
ssh router 'mtd_storage.sh save'
```

Check the success result. This writes the firmware's Storage partition; it is not a command to format or overwrite the firmware. Do not run it repeatedly for every page request or log update.

Reboot only during a suitable maintenance window. After reboot, verify HTTP access, firewall rules, and the PID. Until that test is done, describe persistence as configured, not reboot-tested.

## 9. Extend the example into another service

Keep the transport and handler responsibilities separate. For a new read-only page:

1. Decide exactly which information should be exposed.
2. Collect only those values with bounded, predictable commands.
3. Add a fixed route and render the result.
4. Escape any text inserted into HTML. Numeric `/proc` values used here are controlled; arbitrary hostnames or log messages are not.
5. Return the correct Content-Type. For JSON, correctly encode strings and escape control characters; do not interpolate arbitrary text into JSON.
6. Test malformed paths, unsupported methods, missing data, timeouts, and access restrictions.

Do not add buttons that execute arbitrary commands. A service that changes router settings needs authentication, CSRF protection, authorization, and a more capable HTTP stack. Large applications, many clients, TLS, uploads, or public access should use a maintained server and an appropriate host.

## 10. Stop and remove the service

Use the actual port from the installed configuration. Before killing a PID, inspect its command line; a stale PID may belong to another process:

```sh
ssh router
pid=$(cat /tmp/router-status.pid)
tr '\000' ' ' < "/proc/$pid/cmdline"
```

Only after confirming it is this service's Netcat listener:

```sh
kill "$pid"
rm -f /tmp/router-status.pid
```

Remove the two service calls from the startup and firewall hooks. Preserve unrelated edits made since installation rather than blindly restoring an old backup.

With the old configuration still present:

```sh
. /etc/storage/router-status/config.sh
iptables -D INPUT -p tcp --dport "$PORT" -j RS_STATUS
iptables -F RS_STATUS
iptables -X RS_STATUS
ip6tables -D INPUT -p tcp --dport "$PORT" -j DROP
```

These are service-owned rules; if a matching IPv6 DROP rule existed before installation, preserve it. Inspect rules and remove duplicate service-owned jumps individually if necessary. Do not use `iptables -F INPUT`.

Remove only this project's directory after reviewing its contents, then save storage with the firmware's tool. Verify the port no longer answers and the management panel still works.

## 11. Prepare a public repository

- Keep real configuration, keys, backups, log output, packet captures, and private screenshots out of Git.
- Use placeholders in documentation; never include a real SSH password or private key.
- Review `git status` and `git diff --cached` before committing.
- `.gitignore` does not remove files already tracked. If a secret was committed, remove it from history as appropriate and rotate the exposed credential before publishing.
- State compatibility and testing limits honestly. A sample based on one router is not universal support.
- Choose a license. This project includes MIT; review that choice before publication.

For repository-wide contribution and publication guidance, return to the repository root and read [CONTRIBUTING.md](../../CONTRIBUTING.md). Keep this example under `examples/status-dashboard`; do not initialize a nested Git repository or publish private deployment configuration.

## References

- [BusyBox official applet documentation](https://busybox.net/downloads/BusyBox.html): builds vary; installed `--help` output is the authority for the target.
- [Netfilter / iptables project](https://www.netfilter.org/projects/iptables/index.html): firewall tooling and upstream documentation.
- Device-local firmware scripts and help output were used to establish the observed storage and startup behavior. The hook paths and save command are firmware-specific.
