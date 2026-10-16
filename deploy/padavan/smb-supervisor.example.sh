#!/bin/sh
# Independent USB-aware SMB supervisor. No web environment, firewall or NTP.
# Administrator supplies absolute paths. Run foreground from the USB hook.
set -eu
: "${RS_SMB_BINARY:?set RS_SMB_BINARY to the installed SMB executable}"
: "${RS_SMB_CONFIG:?set RS_SMB_CONFIG to private JSON on protected storage}"
case "$RS_SMB_BINARY:$RS_SMB_CONFIG" in *'
'*|*' '*|*'..'*) echo 'Use absolute paths without spaces or ..' >&2; exit 1;; esac
case "$RS_SMB_BINARY" in /*) ;; *) exit 1;; esac
case "$RS_SMB_CONFIG" in /*) ;; *) exit 1;; esac
if [ "${1:-}" = --check-config ]; then
  exec "$RS_SMB_BINARY" -config "$RS_SMB_CONFIG" -check-config
fi
if [ "$#" -ne 0 ]; then echo 'Only --check-config is supported' >&2; exit 1; fi
# mkdir is atomic: fail on an existing instance rather than trust reused PIDs.
# This directory is distinct from all public-share compatibility paths.
runtime=/tmp/routershelf-smb
umask 077
if ! mkdir "$runtime" 2>/dev/null; then
  echo 'SMB supervisor lock exists; inspect its PID command line before recovery' >&2
  exit 1
fi
child=
cleanup() {
  trap '' INT TERM
  if [ -n "$child" ]; then
    # Only the direct child launched by this shell; never read a PID to kill.
    kill -TERM "$child" 2>/dev/null || :
    wait "$child" 2>/dev/null || :
  fi
  rm -f "$runtime/child.pid" "$runtime/supervisor.pid"
  rmdir "$runtime" 2>/dev/null || :
}
trap 'exit 0' INT TERM
trap cleanup 0
printf '%s\n' "$$" > "$runtime/supervisor.pid"
while :; do
  if [ -x "$RS_SMB_BINARY" ] && [ -f "$RS_SMB_CONFIG" ]; then
    "$RS_SMB_BINARY" -config "$RS_SMB_CONFIG" &
    child=$!
    printf '%s\n' "$child" > "$runtime/child.pid"
    wait "$child" || :
    child=
    rm -f "$runtime/child.pid"
  fi
  # The binary verifies exact mount presence before binding, never mkdirs a
  # missing share, and exits on mount/root replacement. Retry is independent.
  sleep 10 &
  child=$!
  wait "$child" || :
  child=
done
