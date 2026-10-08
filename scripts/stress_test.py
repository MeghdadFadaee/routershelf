#!/usr/bin/env python3
"""Bounded, read-only RouterShelf load test; Python standard library only."""
import argparse
import collections
import concurrent.futures
import csv
import datetime
import http.client
import json
import math
import os
from pathlib import Path
import re
import random
import shlex
import signal
import ssl
import subprocess
import threading
import time
import urllib.parse


def read_env(path):
  values = {}
  if not path.exists():
    return values
  for line in path.read_text().splitlines():
    line = line.strip()
    if not line or line.startswith('#'):
      continue
    if line.startswith('export '):
      line = line[7:]
    key, sep, value = line.partition('=')
    if not sep or not re.fullmatch(r'RS_[A-Z0-9_]+', key):
      continue
    words = shlex.split(value, comments=True)
    if len(words) != 1 or '$' in words[0] or '`' in words[0]:
      raise ValueError('Stress runner requires literal env values; shell code is never executed')
    values[key] = words[0]
  return values


def percentile(values, fraction):
  if not values:
    return None
  ordered = sorted(values)
  return ordered[max(0, math.ceil(len(ordered) * fraction) - 1)]


class Stop:
  def __init__(self):
    self.event = threading.Event()
    self.reason = None
    self.lock = threading.Lock()

  def abort(self, reason):
    with self.lock:
      if self.reason is None:
        self.reason = reason
    self.event.set()


class Client:
  def __init__(self, base_url, timeout=5, max_bytes=2 * 1024 * 1024, fresh=False):
    self.url = urllib.parse.urlsplit(base_url)
    self.timeout = timeout
    self.max_bytes = max_bytes
    self.fresh = fresh
    self.conn = None

  def close(self):
    if self.conn:
      self.conn.close()
      self.conn = None

  def request(self, path, method='GET', headers=None, expected=(200,)):
    start = time.monotonic()
    status = None
    received = 0
    response_headers = {}
    error = None
    try:
      if self.conn is None:
        cls = http.client.HTTPSConnection if self.url.scheme == 'https' else http.client.HTTPConnection
        extra = {'context': ssl.create_default_context()} if self.url.scheme == 'https' else {}
        self.conn = cls(self.url.hostname, self.url.port, timeout=self.timeout, **extra)
      self.conn.request(method, path, headers={'User-Agent': 'RouterShelfStress/1', **(headers or {})})
      if self.conn.sock:
        self.conn.sock.settimeout(max(.001, start + self.timeout - time.monotonic()))
      response = self.conn.getresponse()
      status = response.status
      response_headers = {k.lower(): v for k, v in response.getheaders()}
      body_deadline = start + self.timeout
      while True:
        remaining = body_deadline - time.monotonic()
        if remaining <= 0:
          raise TimeoutError('overall request deadline exceeded')
        if self.conn.sock:
          self.conn.sock.settimeout(remaining)
        chunk = response.read1(min(65536, self.max_bytes + 1 - received))
        if not chunk:
          break
        received += len(chunk)
        if received > self.max_bytes:
          raise ValueError('response exceeded configured byte cap')
      response.close()
      if status not in expected:
        error = 'unexpected HTTP status'
      if response.will_close or self.fresh:
        self.close()
    except Exception as exc:
      error = type(exc).__name__ + ': ' + str(exc)[:160]
      self.close()
    return {'method': method, 'path': path, 'status': status, 'bytes': received,
            'ms': round((time.monotonic() - start) * 1000, 3), 'error': error,
            'headers': response_headers}


def validate_url(url):
  parsed = urllib.parse.urlsplit(url)
  if parsed.scheme not in ('http', 'https') or not parsed.hostname or parsed.username or parsed.password:
    raise ValueError('Use an HTTP(S) URL without credentials')
  if parsed.path not in ('', '/') or parsed.query or parsed.fragment:
    raise ValueError('Base URL must be the origin; choose pages with --html-path/--download-path')
  _ = parsed.port
  return url.rstrip('/')


def validate_path(path):
  parsed = urllib.parse.urlsplit(path)
  if not path.startswith('/') or parsed.scheme or parsed.netloc or parsed.fragment or any(ord(c) < 32 for c in path):
    raise ValueError('Request paths must be local absolute URL paths, without a fragment')
  return path


def remote_sample(alias, runtime):
  if not re.fullmatch(r'[A-Za-z0-9_.@:-]+', alias) or alias.startswith('-'):
    raise ValueError('Invalid SSH alias')
  if not re.fullmatch(r'/[A-Za-z0-9_./-]+', runtime):
    raise ValueError('Invalid remote runtime directory')
  pidfile = shlex.quote(runtime.rstrip('/') + '/public-share-child.pid')
  command = (
    "head -n 1 /proc/stat; awk '/^cpu[0-9]+ /{n++} END{print \"cores\",n}' /proc/stat; "
    "head -n 1 /proc/loadavg; "
    "awk '/^(MemTotal|MemFree|Buffers|Cached):/{print}' /proc/meminfo; "
    "pid=$(cat " + pidfile + " 2>/dev/null); case \"$pid\" in ''|*[!0-9]*) exit 2;; esac; "
    "cat /proc/$pid/stat; awk '/^(VmRSS|Threads):/{print}' /proc/$pid/status"
  )
  result = subprocess.run(['ssh', '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=4', alias, command],
                          capture_output=True, text=True, timeout=7)
  if result.returncode:
    raise RuntimeError('SSH telemetry failed; check key login and service PID')
  sample = {'time': datetime.datetime.now(datetime.timezone.utc).isoformat()}
  memory = {}
  for line in result.stdout.splitlines():
    fields = line.split()
    if line.startswith('cpu '):
      ticks = list(map(int, fields[1:9]))
      sample['cpu_ticks'] = sum(ticks)
      sample['cpu_idle_ticks'] = ticks[3] + ticks[4]
    elif line.startswith('cores '):
      sample['cores'] = int(fields[1])
    elif fields and fields[0] in ('MemTotal:', 'MemFree:', 'Buffers:', 'Cached:', 'VmRSS:', 'Threads:'):
      memory[fields[0].rstrip(':')] = int(fields[1])
    elif re.match(r'^\d+ \(', line):
      # Process names may contain spaces; fields after the last ')' start at stat field 3.
      tail = line.rsplit(')', 1)[1].split()
      sample['pid'] = int(line.split(' ', 1)[0])
      sample['process_ticks'] = int(tail[11]) + int(tail[12])
    elif re.match(r'^\d+\.\d+ ', line):
      sample['load1'] = float(fields[0])
  sample['rss_mib'] = memory['VmRSS'] / 1024
  sample['free_reclaimable_estimate_mib'] = sum(memory[k] for k in ('MemFree', 'Buffers', 'Cached')) / 1024
  sample['memory_total_mib'] = memory['MemTotal'] / 1024
  sample['threads'] = memory['Threads']
  if 'pid' not in sample or 'cpu_ticks' not in sample:
    raise RuntimeError('Incomplete SSH telemetry')
  return sample


def monitor(args, stop, samples, finished):
  previous = None
  failures = 0
  high_cpu = 0
  while not finished.is_set() and not stop.event.is_set():
    try:
      current = remote_sample(args.ssh, args.remote_runtime_dir)
      failures = 0
      if previous:
        if current['pid'] != previous['pid']:
          stop.abort('Service PID changed during test (restart or external update)')
        delta = current['cpu_ticks'] - previous['cpu_ticks']
        if delta > 0:
          current['system_cpu_percent'] = round(100 * (1 - (current['cpu_idle_ticks'] - previous['cpu_idle_ticks']) / delta), 2)
          if current['pid'] == previous['pid']:
            current['process_cpu_percent_of_device'] = round(100 * (current['process_ticks'] - previous['process_ticks']) / delta, 2)
      samples.append(current)
      if current['rss_mib'] > args.max_rss_mib:
        stop.abort('Process RSS crossed configured threshold')
      if current['free_reclaimable_estimate_mib'] < args.min_memory_mib:
        stop.abort('Free plus buffers/cache estimate crossed configured threshold')
      high_cpu = high_cpu + 1 if current.get('system_cpu_percent', 0) > args.max_cpu_percent else 0
      if high_cpu >= 3:
        stop.abort('System CPU stayed above threshold for three samples')
      previous = current
    except Exception as exc:
      failures += 1
      samples.append({'time': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'error': str(exc)})
      if failures >= 3:
        stop.abort('SSH telemetry failed three consecutive times')
    finished.wait(args.monitor_interval)


def phase(args, workers, seconds, endpoints, stop):
  start = time.monotonic()
  deadline = start + seconds
  rate_lock = threading.Lock()
  next_request = start
  records = []
  record_lock = threading.Lock()

  def worker(worker_id):
    nonlocal next_request
    client = Client(args.url, args.timeout, args.max_bytes, args.fresh_connections)
    index = worker_id
    rng = random.Random(worker_id)
    try:
      while not stop.event.is_set() and time.monotonic() < deadline:
        with rate_lock:
          now = time.monotonic()
          reserved = max(now, next_request)
          next_request = reserved + 1 / args.max_rps
        wait = reserved - time.monotonic()
        if reserved >= deadline or stop.event.wait(max(0, wait)):
          break
        path, method, headers = endpoints[index % len(endpoints)]
        index += 1
        if args.random_ranges and path == args.download_path:
          offset = rng.randrange(max(1, args.download_size - args.range_bytes + 1))
          end = min(args.download_size - 1, offset + args.range_bytes - 1)
          headers = {'Range': 'bytes={}-{}'.format(offset, end)}
        record = client.request(path, method, headers, (206,) if headers.get('Range') else (200,))
        with record_lock:
          records.append(record)
          if len(records) >= 20:
            recent = records[-20:]
            if sum(r['error'] is not None for r in recent) / len(recent) > args.max_error_ratio:
              stop.abort('Recent request error ratio crossed configured threshold')
    finally:
      client.close()

  with concurrent.futures.ThreadPoolExecutor(max_workers=workers) as pool:
    list(pool.map(worker, range(workers)))
  elapsed = time.monotonic() - start
  successes = [r for r in records if not r['error']]
  latency = [r['ms'] for r in successes]
  return {'workers': workers, 'requested_seconds': seconds, 'elapsed_seconds': round(elapsed, 3),
          'requests': len(records), 'successes': len(successes), 'errors': len(records) - len(successes),
          'rps': round(len(records) / elapsed, 3),
          'mib_per_second': round(sum(r['bytes'] for r in successes) / elapsed / 1048576, 3),
          'p50_ms': percentile(latency, .5), 'p95_ms': percentile(latency, .95), 'p99_ms': percentile(latency, .99),
          'statuses': dict(collections.Counter(str(r['status']) for r in records)), 'records': records}


def endpoints_for(args):
  endpoints = [('/', 'GET', {}), ('/_nais/autoindex.css', 'GET', {}), ('/_nais/autoindex.js', 'GET', {}), ('/', 'HEAD', {})]
  for path in args.html_path + args.asset_path:
    endpoints.append((path, 'GET', {}))
  if args.download_path:
    endpoints.append((args.download_path, 'GET', {'Range': 'bytes=0-' + str(args.range_bytes - 1)}))
  return endpoints


def health(args):
  client = Client(args.url, args.timeout, args.max_bytes)
  checks = []
  try:
    for path, method, headers in endpoints_for(args):
      checks.append(client.request(path, method, headers, (200, 206)))
    checks.append(client.request('/.env', expected=(404,)))
    checks.append(client.request('/../etc/passwd', expected=(404,)))
    for path in args.html_path:
      html = next(c for c in checks if c['path'] == path and c['method'] == 'GET')
      if not html['error'] and (not html['headers'].get('content-type', '').startswith('text/html') or 'attachment' in html['headers'].get('content-disposition', '')):
        html['error'] = 'HTML MIME/disposition regression'
    if args.download_path:
      download = next(c for c in checks if c['path'] == args.download_path)
      if not download['error'] and (download['status'] != 206 or not download['headers'].get('content-range', '').startswith('bytes 0-')):
        download['error'] = 'Byte-range response regression'
    if args.random_ranges:
      size_check = client.request(args.download_path, 'HEAD')
      try:
        args.download_size = int(size_check['headers'].get('content-length', '0'))
        if args.download_size <= 0:
          size_check['error'] = 'Random ranges require a nonempty file with Content-Length'
      except ValueError:
        size_check['error'] = 'Invalid download Content-Length'
      checks.append(size_check)
    if args.http_url:
      redirect_client = Client(args.http_url, args.timeout, args.max_bytes)
      try:
        redirect = redirect_client.request('/', 'HEAD', expected=(301,302,307,308))
        if not redirect['error'] and redirect['headers'].get('location') != args.url + '/':
          redirect['error'] = 'Unexpected HTTPS redirect target'
        redirect['path'] = 'HTTP redirect /'
        checks.append(redirect)
      finally:
        redirect_client.close()

  finally:
    client.close()
  return checks


def write_report(output, report):
  valid_samples = [s for s in report['telemetry'] if 'rss_mib' in s]
  report['resource_summary'] = {
    'peak_rss_mib': max((s['rss_mib'] for s in valid_samples), default=None),
    'peak_system_cpu_percent': max((s['system_cpu_percent'] for s in valid_samples if 'system_cpu_percent' in s), default=None),
    'peak_process_cpu_percent_of_device': max((s['process_cpu_percent_of_device'] for s in valid_samples if 'process_cpu_percent_of_device' in s), default=None),
    'minimum_free_reclaimable_estimate_mib': min((s['free_reclaimable_estimate_mib'] for s in valid_samples), default=None),
    'observed_pids': sorted(set(s['pid'] for s in valid_samples)),
  }
  output.mkdir(parents=True, exist_ok=False)
  output.chmod(0o700)
  (output / 'report.json').write_text(json.dumps(report, indent=2) + '\n')
  with (output / 'requests.csv').open('w', newline='') as f:
    writer = csv.DictWriter(f, fieldnames=['phase', 'workers', 'method', 'path', 'status', 'bytes', 'ms', 'error'])
    writer.writeheader()
    for index, stage in enumerate(report['phases']):
      for record in stage['records']:
        writer.writerow({'phase': index + 1, 'workers': stage['workers'], **{k: record[k] for k in ['method', 'path', 'status', 'bytes', 'ms', 'error']}})
  with (output / 'telemetry.csv').open('w', newline='') as f:
    writer = csv.DictWriter(f, fieldnames=['time', 'pid', 'rss_mib', 'system_cpu_percent', 'process_cpu_percent_of_device', 'free_reclaimable_estimate_mib', 'threads', 'load1', 'error'], extrasaction='ignore')
    writer.writeheader()
    writer.writerows(report['telemetry'])
  rows = ['# RouterShelf stress report', '', '**Outcome:** ' + report['outcome'], '',
          'Target: ' + report['url'], '', 'Connection mode: ' + report['connection_mode'], '',
          '| Workers | Requests | Errors | req/s | MiB/s | p50 ms | p95 ms | p99 ms |',
          '| --- | --- | --- | --- | --- | --- | --- | --- |']
  for stage in report['phases']:
    rows.append('| ' + ' | '.join(str(stage[k]) for k in ['workers', 'requests', 'errors', 'rps', 'mib_per_second', 'p50_ms', 'p95_ms', 'p99_ms']) + ' |')
  if valid_samples:
    summary = report['resource_summary']
    rows += ['', '## Router resources', '',
             'Peak service RSS: {} MiB.'.format(round(summary['peak_rss_mib'], 2)),
             'Peak sampled system CPU: {}%.'.format(summary['peak_system_cpu_percent']),
             'Peak sampled service CPU, total device capacity: {}%.'.format(summary['peak_process_cpu_percent_of_device']),
             'Minimum free + buffers/cache estimate: {} MiB.'.format(round(summary['minimum_free_reclaimable_estimate_mib'], 2))]
  else:
    rows += ['', 'Router resources were not collected.']
  if report['stop_reason']:
    rows += ['', 'Stopped because: ' + report['stop_reason']]
  rows += ['', 'Telemetry and per-request details are in report.json and requests.csv.',
           'CPU percentages use total device CPU capacity. Free memory is an estimate including buffers/cache.',
           'Reports contain deployment URLs and paths; keep them private.']
  (output / 'report.md').write_text('\n'.join(rows) + '\n')


def arguments(argv=None):
  parser = argparse.ArgumentParser(description=__doc__)
  parser.add_argument('--env', type=Path, default=Path('.env'))
  parser.add_argument('--url')
  parser.add_argument('--levels', default='1,2,4,8')
  parser.add_argument('--seconds', type=float, default=15)
  parser.add_argument('--cooldown', type=float, default=5)
  parser.add_argument('--soak', type=float, default=0, help='Additional stage at highest concurrency')
  parser.add_argument('--max-rps', type=float, default=20, help='Global request-rate ceiling, shared across workers')
  parser.add_argument('--timeout', type=float, default=5)
  parser.add_argument('--max-bytes', type=int, default=2 * 1024 * 1024)
  parser.add_argument('--range-bytes', type=int, default=1024 * 1024)
  parser.add_argument('--html-path', action='append', default=[], help='Repeat for directory index and direct HTML checks')
  parser.add_argument('--asset-path', action='append', default=[])
  parser.add_argument('--http-url', help='HTTP origin for redirect check; derived for HTTPS targets')
  parser.add_argument('--download-path')
  parser.add_argument('--random-ranges', action='store_true', help='Read deterministic pseudorandom file sections instead of repeating its start')
  parser.add_argument('--fresh-connections', action='store_true', help='New connection/TLS handshake for every request')
  parser.add_argument('--ssh', help='Optional SSH alias for router telemetry; key authentication only')
  parser.add_argument('--remote-runtime-dir')
  parser.add_argument('--monitor-interval', type=float, default=3)
  parser.add_argument('--max-rss-mib', type=float, default=48)
  parser.add_argument('--min-memory-mib', type=float, default=8)
  parser.add_argument('--max-cpu-percent', type=float, default=90)
  parser.add_argument('--max-error-ratio', type=float, default=.2)
  parser.add_argument('--output', type=Path)
  parser.add_argument('--dry-run', action='store_true', help='Show plan without any HTTP or SSH connection')
  args = parser.parse_args(argv)
  try:
    env = read_env(args.env)
    domain = os.environ.get('RS_DOMAIN') or env.get('RS_DOMAIN')
    port = os.environ.get('RS_PUBLIC_HTTPS_PORT') or env.get('RS_PUBLIC_HTTPS_PORT', '443')
    args.url = validate_url(args.url or ('https://' + domain + (':' + port if port != '443' else '') if domain else ''))
    if args.http_url:
      args.http_url = validate_url(args.http_url)
    elif args.url.startswith('https://'):
      http_port = env.get('RS_PUBLIC_HTTP_PORT', '80')
      hostname = urllib.parse.urlsplit(args.url).hostname
      if ':' in hostname:
        hostname = '[' + hostname + ']'
      args.http_url = validate_url('http://' + hostname + (':' + http_port if http_port != '80' else ''))
    args.levels = [int(n) for n in args.levels.split(',')]
    if not args.levels or args.levels != sorted(set(args.levels)) or not all(1 <= n <= 32 for n in args.levels):
      raise ValueError('Levels must be increasing distinct integers between 1 and 32')
    if not 0 < args.seconds <= 300 or not 0 <= args.soak <= 600 or not 0 <= args.cooldown <= 60:
      raise ValueError('Phase <=300s, soak <=600s, cooldown <=60s; durations must be nonnegative')
    if not 0 < args.max_rps <= 500 or not 0 < args.timeout <= 15 or not 1 <= args.monitor_interval <= 30:
      raise ValueError('Rate <=500 req/s, timeout <=15s, telemetry interval 1..30s')
    if not 0 < args.max_error_ratio <= 1 or not 0 < args.max_cpu_percent <= 100 or args.max_rss_mib <= 0 or args.min_memory_mib < 0:
      raise ValueError('Invalid resource/error threshold')
    if not 1 <= args.range_bytes <= args.max_bytes <= 16 * 1024 * 1024:
      raise ValueError('Require 1 <= range-bytes <= max-bytes <=16MiB')
    if args.random_ranges and not args.download_path:
      raise ValueError('--random-ranges requires --download-path')
    for path in args.html_path + args.asset_path + ([args.download_path] if args.download_path else []):
      if path:
        validate_path(path)
    args.remote_runtime_dir = args.remote_runtime_dir or env.get('RS_RUNTIME_DIR', '/tmp')
    if args.output is None:
      stamp = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')
      args.output = Path('outputs/stress') / (stamp + '-' + str(os.getpid()))
    if args.output.exists():
      raise ValueError('Output directory already exists; refusing to overwrite results')
  except (ValueError, OSError) as exc:
    parser.error(str(exc))
  return args


def main(argv=None):
  args = arguments(argv)
  plan = {'url': args.url, 'levels': args.levels, 'seconds_per_phase': args.seconds, 'cooldown_seconds': args.cooldown,
          'soak_seconds': args.soak, 'max_rps': args.max_rps, 'fresh_connections': args.fresh_connections,
          'ssh_telemetry': bool(args.ssh), 'http_redirect_origin': args.http_url,
          'random_ranges': args.random_ranges, 'response_byte_cap': args.max_bytes, 'range_bytes': args.range_bytes,
          'stop_thresholds': {'rss_mib': args.max_rss_mib, 'memory_estimate_mib': args.min_memory_mib,
                              'cpu_percent': args.max_cpu_percent, 'recent_error_ratio': args.max_error_ratio}, 'endpoints': [p for p, _, _ in endpoints_for(args)], 'output': str(args.output)}
  print(json.dumps(plan, indent=2), flush=True)
  if args.dry_run:
    return 0
  stop = Stop()
  finished = threading.Event()
  samples = []
  report = {'url': args.url, 'started_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
            'plan': plan, 'connection_mode': 'fresh TLS/connection per request' if args.fresh_connections else 'per-worker keep-alive',
            'before': [], 'after': [], 'phases': [], 'telemetry': samples}
  old_handler = signal.signal(signal.SIGINT, lambda *_: stop.abort('Interrupted by user'))
  telemetry = None
  try:
    report['before'] = health(args)
    if any(c['error'] for c in report['before']):
      stop.abort('Preflight checks failed; no load stages started')
    if args.ssh and not stop.event.is_set():
      # Check monitoring before starting any load.
      samples.append(remote_sample(args.ssh, args.remote_runtime_dir))
      telemetry = threading.Thread(target=monitor, args=(args, stop, samples, finished), daemon=True)
      telemetry.start()
    for workers in args.levels:
      if stop.event.is_set():
        break
      stage = phase(args, workers, args.seconds, endpoints_for(args), stop)
      report['phases'].append(stage)
      print('workers={} requests={} errors={} p95={}ms'.format(workers, stage['requests'], stage['errors'], stage['p95_ms']), flush=True)
      if stop.event.wait(args.cooldown):
        break
    if args.soak and not stop.event.is_set():
      report['phases'].append(phase(args, args.levels[-1], args.soak, endpoints_for(args), stop))
      stop.event.wait(args.cooldown)
    report['after'] = health(args)
  except Exception as exc:
    stop.abort(type(exc).__name__ + ': ' + str(exc))
  finally:
    finished.set()
    if telemetry:
      telemetry.join(timeout=8)
    signal.signal(signal.SIGINT, old_handler)
    report['stop_reason'] = stop.reason
    report['finished_utc'] = datetime.datetime.now(datetime.timezone.utc).isoformat()
    failed = bool(stop.reason) or any(c['error'] for c in report['after']) or any(p['errors'] for p in report['phases'])
    report['outcome'] = 'incomplete/failed checks' if failed else 'completed configured workload'
    write_report(args.output, report)
    print('Report: ' + str(args.output / 'report.md'), flush=True)
  return 1 if failed else 0


if __name__ == '__main__':
  raise SystemExit(main())
