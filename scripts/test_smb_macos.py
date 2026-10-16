#!/usr/bin/env python3
"""Opt-in native macOS SMB smoke test, localhost + temporary data only.

Usage: python3 scripts/test_smb_macos.py /absolute/path/to/routershelf-smb [--read-only] [--disk-root]
Uses a disposable test account/password, never a router or private configuration.
Requires macOS mount_smbfs and permission to mount a local network filesystem.
"""
import hashlib
import json
import os
from pathlib import Path
import platform
import socket
import subprocess
import sys
import tempfile
import time


def main():
  if platform.system() != "Darwin" or len(sys.argv) < 2:
    raise SystemExit("Use on macOS with one absolute local executable path")
  options=set(sys.argv[2:])
  if options-{"--read-only","--disk-root"}: raise SystemExit("Only --read-only and --disk-root are supported")
  readonly="--read-only" in options
  diskroot="--disk-root" in options
  binary = Path(sys.argv[1]).resolve(strict=True)
  password = "RouterShelf-local-test-only-123"  # public synthetic credential
  digest = subprocess.check_output(
    [str(binary), "-hash-password"], input=password.encode(), timeout=10
  ).decode().strip()
  with tempfile.TemporaryDirectory(prefix="routershelf-smb-native-") as temp:
    base = Path(temp).resolve()
    root, mount = base / "storage", base / "client"
    root.mkdir()
    (root / "read-only-probe.txt").write_text("RouterShelf read-only probe")
    mount.mkdir()
    with socket.socket() as probe:
      probe.bind(("127.0.0.1", 0))
      port = probe.getsockname()[1]
    cfg = base / "smb.json"
    settings = dict(listen=f"127.0.0.1:{port}", storage_mount=str(base),
            root=str(root), user="storage", nt_hash=digest,
            read_only=readonly, allow=["127.0.0.1/32"],
            uid=os.getuid(), gid=os.getgid())
    if diskroot:
      settings.update(storage_mount=str(root),allow_storage_root=True,allow_public_overlap=True)
      (root/".smb-service").mkdir()
      (root/".smb-service"/"private-sentinel").write_text("must stay private")
    cfg.write_text(json.dumps(settings))
    cfg.chmod(0o600)
    subprocess.run([str(binary), "-config", str(cfg), "-check-config"],
           check=True, timeout=10)
    proc = subprocess.Popen([str(binary), "-config", str(cfg)],
                stdout=subprocess.DEVNULL)
    mounted = False
    try:
      deadline = time.monotonic() + 10
      while True:
        if proc.poll() is not None:
          raise RuntimeError("local SMB server exited")
        try:
          with socket.create_connection(("127.0.0.1", port), timeout=1):
            break
        except OSError:
          if time.monotonic() > deadline:
            raise
          time.sleep(0.1)
      # Password in this URL is synthetic and used for this temp test only.
      # Interactive product instructions use Finder and a hidden prompt.
      bad = subprocess.run(["/sbin/mount_smbfs", "-N",
                 f"//storage:wrong-fixture-password@127.0.0.1:{port}/Storage", str(mount)],
                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=20)
      if bad.returncode == 0:
        mounted = True
        raise RuntimeError("native client accepted an incorrect password")
      print("PASS native macOS rejects incorrect password", flush=True)
      url = f"//storage:{password}@127.0.0.1:{port}/Storage"
      subprocess.run(["/sbin/mount_smbfs", "-N", "-o", "nobrowse", url, str(mount)],
             check=True, timeout=30)
      mounted = True
      subprocess.run([sys.executable, __file__, "--read-only-operations" if readonly else "--operations", str(mount)],
             check=True, timeout=30)
    finally:
      try:
        if mounted:
          subprocess.run(["/sbin/umount", str(mount)], check=True, timeout=30)
      finally:
        proc.terminate()
        try:
          proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
          proc.kill()
          proc.wait(timeout=10)


def operations(mount):
  if ".smb-service" in {p.name for p in mount.iterdir()}: raise RuntimeError("private service directory exposed")
  try:
    (mount/".smb-service"/"private-sentinel").read_bytes()
  except OSError: pass
  else: raise RuntimeError("private service data exposed")
  (mount / "folder").mkdir()
  block = b"RouterShelf-integrity-probe-" * 2048
  expected = hashlib.sha256()
  with (mount / "folder" / "upload.partial").open("wb") as out:
    for _ in range(256):
      out.write(block)
      expected.update(block)
  (mount / "folder" / "upload.partial").rename(mount / "download.bin")
  actual = hashlib.sha256()
  with (mount / "download.bin").open("rb") as source:
    for chunk in iter(lambda: source.read(65536), b""):
      actual.update(chunk)
  if actual.digest() != expected.digest():
    raise RuntimeError("native transfer checksum mismatch")
  (mount / "download.bin").unlink()
  (mount / "folder").rmdir()
  print("PASS native macOS mount, mkdir, upload, rename/move, download/hash, delete")
  print("Finder GUI, metadata, interruption and router resource checks remain separate")


def readonly_operations(mount):
  probe = mount / "read-only-probe.txt"
  if probe.read_text() != "RouterShelf read-only probe":
    raise RuntimeError("read-only probe read failed")
  for action in [lambda: probe.write_text("forbidden"),
         lambda: (mount / "forbidden-folder").mkdir(),
         lambda: probe.rename(mount / "forbidden-rename"),
         lambda: probe.unlink()]:
    try:
      action()
    except OSError:
      continue
    raise RuntimeError("native read-only mutation succeeded")
  print("PASS native macOS read-only read; write/mkdir/rename/delete refused", flush=True)


if __name__ == "__main__":
  if len(sys.argv) == 3 and sys.argv[1] == "--operations":
    operations(Path(sys.argv[2]))
  elif len(sys.argv) == 3 and sys.argv[1] == "--read-only-operations":
    readonly_operations(Path(sys.argv[2]))
  else:
    main()
