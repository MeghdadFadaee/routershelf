import contextlib
import http.server
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import threading
import unittest
from unittest.mock import patch
from types import SimpleNamespace
import sys

spec = importlib.util.spec_from_file_location('stress', Path(__file__).parents[1] / 'stress_test.py')
stress = importlib.util.module_from_spec(spec)
spec.loader.exec_module(stress)


class Handler(http.server.BaseHTTPRequestHandler):
  protocol_version = 'HTTP/1.1'

  def log_message(self, *args):
    pass

  def do_HEAD(self):
    self.do_GET()

  def do_GET(self):
    missing = self.path in ('/.env', '/../etc/passwd')
    body = b'<h1>Test</h1>' if self.path == '/page.html' else b'healthy'
    is_range = self.path == '/download.bin' and self.headers.get('Range')
    self.send_response(206 if is_range else (404 if missing else 200))
    if is_range:
      self.send_header('Content-Range', 'bytes 0-6/7')
    self.send_header('Content-Length', str(len(body)))
    self.send_header('Content-Type', 'text/html' if self.path == '/page.html' else 'text/plain')
    self.end_headers()
    if self.command != 'HEAD':
      self.wfile.write(body)


class QuietServer(http.server.ThreadingHTTPServer):
  def handle_error(self, request, client_address):
    if not isinstance(sys.exc_info()[1], (ConnectionResetError, BrokenPipeError)):
      super().handle_error(request, client_address)


class StressTests(unittest.TestCase):
  @classmethod
  def setUpClass(cls):
    cls.server = QuietServer(('127.0.0.1', 0), Handler)
    cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
    cls.thread.start()
    cls.url = 'http://127.0.0.1:' + str(cls.server.server_port)

  @classmethod
  def tearDownClass(cls):
    cls.server.shutdown()
    cls.server.server_close()
    cls.thread.join()

  def test_full_local_run_and_report(self):
    with tempfile.TemporaryDirectory() as temp, contextlib.redirect_stdout(io.StringIO()):
      output = Path(temp) / 'report'
      code = stress.main(['--url', self.url, '--env', str(Path(temp) / 'absent.env'),
                          '--levels', '1,2', '--seconds', '.15', '--cooldown', '0',
                          '--max-rps', '40', '--html-path', '/page.html', '--output', str(output)])
      report = json.loads((output / 'report.json').read_text())
      self.assertEqual(code, 0)
      self.assertEqual(len(report['phases']), 2)
      self.assertTrue(all(stage['requests'] > 0 and stage['errors'] == 0 for stage in report['phases']))
      self.assertTrue((output / 'requests.csv').exists())
      self.assertTrue((output / 'report.md').exists())
      self.assertEqual(report['before'][-1]['status'], 404)

  def test_client_status_and_body_cap(self):
    client = stress.Client(self.url, max_bytes=3)
    try:
      result = client.request('/')
      self.assertIn('byte cap', result['error'])
    finally:
      client.close()
    client = stress.Client(self.url)
    try:
      self.assertIsNotNone(client.request('/.env')['error'])
      self.assertIsNone(client.request('/.env', expected=(404,))['error'])
    finally:
      client.close()

  def test_range_and_error_abort(self):
    args = stress.arguments(['--url', self.url, '--download-path', '/download.bin', '--random-ranges'])
    self.assertTrue(all(not c['error'] for c in stress.health(args)))
    args.max_rps = 100
    stop = stress.Stop()
    stage = stress.phase(args, 2, 1, [('/.env', 'GET', {})], stop)
    self.assertTrue(stop.event.is_set())
    self.assertGreaterEqual(stage['errors'], 20)
    self.assertIn('error ratio', stop.reason)

  def test_env_is_not_executed(self):
    with tempfile.TemporaryDirectory() as temp:
      path = Path(temp) / '.env'
      path.write_text('RS_DOMAIN=files.example.com\nRS_RUNTIME_DIR="/tmp" # note\n')
      self.assertEqual(stress.read_env(path)['RS_DOMAIN'], 'files.example.com')
      path.write_text('RS_DOMAIN=$(touch /tmp/not-executed)\n')
      with self.assertRaises(ValueError):
        stress.read_env(path)

  def test_dry_run_does_not_connect(self):
    with tempfile.TemporaryDirectory() as temp, contextlib.redirect_stdout(io.StringIO()):
      output = Path(temp) / 'results'
      self.assertEqual(stress.main(['--url', 'https://files.example.com', '--env', str(Path(temp) / 'absent'),
                                   '--output', str(output), '--dry-run']), 0)
      self.assertFalse(output.exists())

  def test_telemetry_parser_and_memory_stop(self):
    tail = ['S'] + ['0'] * 20
    tail[11], tail[12] = '12', '3'
    stdout = 'cpu 100 0 10 500 2 0 0 0 0 0\ncores 4\n0.01 0.02 0.03 1/60 9\nMemTotal: 126280 kB\nMemFree: 16000 kB\nBuffers: 26000 kB\nCached: 36000 kB\n123 (name with spaces) ' + ' '.join(tail) + '\nVmRSS: 8192 kB\nThreads: 5\n'
    with patch.object(stress.subprocess, 'run', return_value=SimpleNamespace(returncode=0, stdout=stdout)):
      sample = stress.remote_sample('router', '/tmp')
    self.assertEqual(sample['process_ticks'], 15)
    self.assertEqual(sample['rss_mib'], 8)
    args = SimpleNamespace(ssh='router', remote_runtime_dir='/tmp', max_rss_mib=7,
                           min_memory_mib=8, max_cpu_percent=90, monitor_interval=.01)
    stop = stress.Stop()
    samples = []
    with patch.object(stress, 'remote_sample', return_value=sample):
      stress.monitor(args, stop, samples, threading.Event())
    self.assertTrue(stop.event.is_set())
    self.assertIn('RSS', stop.reason)

  def test_limits_percentiles_and_paths(self):
    self.assertEqual(stress.percentile([40, 10, 20, 30], .95), 40)
    self.assertIsNone(stress.percentile([], .95))
    for path in ['https://other.example/', '//other.example/', '/x\nheader']:
      with self.assertRaises(ValueError):
        stress.validate_path(path)
    with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit):
      stress.arguments(['--url', self.url, '--levels', '64'])


if __name__ == '__main__':
  unittest.main()
