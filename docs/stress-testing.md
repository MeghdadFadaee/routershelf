# Stress testing RouterShelf

The standard-library Python runner lives at `scripts/stress_test.py`. It exercises read-only HTTP/HTTPS requests from your computer; it is not installed on the router. Use Python 3.10+ and, optionally, an OpenSSH client with authorized key login. Test only an endpoint you administer.

The runner was validated against a temporary localhost server and synthetic telemetry. **No live-router stress workload has been run yet.** A build or runner test does not establish router capacity.

## What it covers

- Preflight and postflight root GET/HEAD, NAIS assets, hidden-path/traversal rejection, and HTTP-to-HTTPS redirect for HTTPS targets.
- Optional directory index and direct HTML requests, checking HTML MIME type and absence of attachment disposition.
- Optional additional asset paths, which are requested explicitly rather than discovered by crawling.
- Optional byte-range downloads; preflight checks for HTTP 206 and Content-Range. Pseudorandom ranges can spread reads across a larger file.
- Incremental concurrency, cooldown periods, optional sustained load, and keep-alive versus fresh connections/TLS handshakes.
- Request counts, status/error counts, completed-response p50/p95/p99 latency, request rate and application-byte throughput.
- Optional SSH samples of service PID/RSS/thread count, total system CPU, process share of total device CPU capacity, load average and estimated free/reclaimable memory.

All workload requests use GET/HEAD. The runner does not upload/delete files, alter firewall/configuration, clear caches, reboot, or modify the service. It does not execute browser JavaScript, simulate a full browser, test reboot/renewal, or replace a disk-health check. HTTPS uses normal certificate validation; there is no insecure TLS switch.

## Start with an offline plan

Run from the repository root. The base domain/public port come from private `.env` or exported `RS_*` values; `--url` overrides the target. The reader accepts literal assignments and shell quoting, but **does not source or execute** the env file. Derived shell expressions must be replaced by literal values for this tool.

```sh
python3 scripts/stress_test.py --dry-run
```

A dry run creates no connections or reports. Check the target, request paths and limits before starting. `--ssh router` uses an example SSH alias; substitute your own alias. SSH must address the same router whose service is receiving the load.

## Stage 1: small baseline

```sh
python3 scripts/stress_test.py \
  --ssh router \
  --levels 1,2,4,8 --seconds 15 --cooldown 5 --max-rps 20
```

This is the default four-stage workload, approximately 80 seconds plus pre/post checks and in-flight timeout handling. `--max-rps` is a **global** ceiling shared by all workers, not a per-worker rate. It includes load-stage requests, but not the small pre/post check set. Concurrency increases alone may not increase throughput if this ceiling is already reached.

Without `--ssh`, the HTTP test works but CPU/RSS/memory/PID safeguards cannot operate. Keep other home-network traffic stable so comparisons are meaningful. Telemetry SSH connections add some CPU overhead, particularly when measuring a small router; the system CPU figures include that overhead and other services.

## Stage 2: static pages and real file reads

Use paths that already exist. Do not copy the illustrative names unchanged unless those files are present. The runner will stop at preflight if a requested resource is missing or a check fails.

```sh
python3 scripts/stress_test.py \
  --ssh router --levels 1,2,4,8 --seconds 20 --max-rps 30 \
  --html-path /demo/ --html-path /brochure.html \
  --asset-path /demo/style.css --asset-path /demo/app.js \
  --download-path /sample.bin --random-ranges --range-bytes 1048576
```

`--html-path` and `--asset-path` can repeat. Use URL-encoded paths for spaces/non-ASCII names as needed. Only same-origin absolute request paths are accepted; absolute external URLs are rejected. No links from the requested HTML are automatically fetched.

A file larger than RAM with pseudorandom ranges is more representative of storage reads than repeatedly reading one tiny cached file, but operating-system and USB caches still affect results. The script does not clear them or claim a pure USB benchmark. Choose an existing nonempty public test file; the script will not create one.

## Stage 3: higher concurrency and soak

After baseline results are satisfactory:

```sh
python3 scripts/stress_test.py \
  --ssh router --levels 1,2,4,8,16,32 \
  --seconds 30 --cooldown 10 --max-rps 100 --soak 180
```

This is approximately seven minutes plus checks and in-flight handling. Optional file/page arguments can be added. It tests the configured workload; it does not prove the maximum number of Internet users. Do not interpret the rate ceiling as a measured service limit.

The runner limits concurrency to 32, stage duration to 300 seconds, soak to 600 seconds, global rate to 500 requests/second, request timeout to 15 seconds and response cap to 16 MiB. Defaults are lower. These are tool bounds, not a promise that the router can sustain their maximums.

## Stage 4: separate TLS handshake pressure

```sh
python3 scripts/stress_test.py \
  --ssh router --levels 1,2,4,8 --seconds 15 --max-rps 20 \
  --fresh-connections
```

The default mode reuses one HTTP/1.1 connection per worker within each stage. This mode closes after each request, forcing new connections and HTTPS handshakes. It can be substantially more CPU-intensive. Compare the two runs; do not assume all browser traffic behaves like either extreme.

## Stop conditions and recovery

Ctrl+C requests a controlled stop and still writes a partial report. Already-running requests finish or time out. Cooldown stages issue no load requests; SSH sampling can continue.

Automatic stop defaults:

| Condition | Default |
| --- | --- |
| Failed preflight | No load stages start |
| Errors in last 20 completed load requests | More than 20% |
| Service RSS, with SSH | Above 48 MiB |
| Free + buffers + cache estimate, with SSH | Below 8 MiB |
| Total system CPU, with SSH | Above 90% for 3 measured intervals |
| Service PID changes, with SSH | Stop; possible restart/external update |
| SSH sampling fails | 3 consecutive failures |

Thresholds are adjustable with `--max-error-ratio`, `--max-rss-mib`, `--min-memory-mib`, `--max-cpu-percent`, and `--monitor-interval`. Exceeding a threshold is a reason to stop and review, not automatic proof of a service bug. The memory estimate is not Linux MemAvailable and does not guarantee that every cached byte is immediately reclaimable. CPU percentages use total device capacity, not a one-core 0–400% convention.

If SSH is requested but initial sampling fails, no load starts. Postflight performs only a small health-check set; it does not restart or recover the router. Check the normal operations guide for recovery if the service becomes unavailable. Preserve other Internet and management services.

## Reports and interpretation

Each run creates a new private `outputs/stress/<UTC timestamp>-<PID>/` directory, or the path passed to `--output`. Existing directories are never overwritten. The output directory has mode 0700 on supported filesystems.

- `report.md`: phase summary and outcome.
- `report.json`: full plan, pre/post checks, all request records and SSH samples.
- `requests.csv`: individual request timings, status, bytes and errors.
- `telemetry.csv`: time series of router CPU/RSS/memory/PID values.

Reports contain deployment URLs and request paths. Keep them private; `outputs/` is ignored by Git. Do not store results inside the public USB share. Exit code 0 means the configured run completed with checks passing; nonzero means stopped/incomplete or failed checks. It does not certify unlimited capacity.

Look for: zero errors, stable service PID, passing postflight, latency growth as load rises, RSS trends rather than one peak, CPU recovery during cooldown, and throughput appropriate to the chosen workload. Latencies describe completed responses and include network/TLS/body transfer; failed requests are counted separately. Bytes measure HTTP payload, not TLS/wire overhead. LAN hairpin results do not measure WAN uplink performance; repeat the same plan from an authorized external client to measure that path.

## Testing the runner itself

```sh
make test-stress-tool
```

This starts only a temporary localhost fixture and exercises reports, limits, read-only request behavior, env parsing, HEAD/keep-alive, ranges and synthetic telemetry stop logic. It does not connect to the configured deployment or need SSH. `make test` includes these checks; Python 3.10+ is therefore a development dependency, not a router runtime dependency.
