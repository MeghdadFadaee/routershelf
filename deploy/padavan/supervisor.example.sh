#!/bin/sh
# RouterShelf Padavan supervisor. Only load administrator-owned configuration.
set -eu
ENV_FILE=${ROUTERSHELF_ENV_FILE:-/etc/storage/routershelf.env}
[ -r "$ENV_FILE" ] || { echo "RouterShelf env file missing: $ENV_FILE" >&2; exit 1; }
set -a
. "$ENV_FILE"
set +a
: "${RS_DOMAIN:?RS_DOMAIN is required}" "${RS_LAN_IP:?RS_LAN_IP is required}" "${RS_USB_MOUNT:?RS_USB_MOUNT is required}"
: "${RS_LAN_SUBNET:?RS_LAN_SUBNET is required}" "${RS_LAN_INTERFACE:?RS_LAN_INTERFACE is required}"
RS_PUBLIC_SUBDIR=${RS_PUBLIC_SUBDIR:-public-storage}
RS_SERVICE_SUBDIR=${RS_SERVICE_SUBDIR:-.public-share-service}
RS_ROOT=${RS_ROOT:-$RS_USB_MOUNT/$RS_PUBLIC_SUBDIR}
RS_BINARY=${RS_BINARY:-$RS_USB_MOUNT/$RS_SERVICE_SUBDIR/public-directory}
RS_CERT_CACHE=${RS_CERT_CACHE:-$RS_USB_MOUNT/$RS_SERVICE_SUBDIR/certificates}
RS_CA_BUNDLE=${RS_CA_BUNDLE:-$RS_USB_MOUNT/$RS_SERVICE_SUBDIR/ca-certificates.pem}
RS_HTTP_PORT=${RS_HTTP_PORT:-10080}
RS_HTTPS_PORT=${RS_HTTPS_PORT:-10443}
RS_PUBLIC_ACCESS=${RS_PUBLIC_ACCESS:-false}
RS_DROP_PRIVILEGES=${RS_DROP_PRIVILEGES:-true}
RS_CHECK_INTERVAL=${RS_CHECK_INTERVAL:-10}
RS_MIN_YEAR=${RS_MIN_YEAR:-2026}
RS_MEMORY_LIMIT=${RS_MEMORY_LIMIT:-32MiB}
RS_CPU_LIMIT=${RS_CPU_LIMIT:-2}
RS_NTP_SERVER1=${RS_NTP_SERVER1:-time.cloudflare.com}
RS_NTP_SERVER2=${RS_NTP_SERVER2:-pool.ntp.org}
RS_SUPERVISOR_PATH=${RS_SUPERVISOR_PATH:-/etc/storage/public-share-service.sh}
RS_RUNTIME_DIR=${RS_RUNTIME_DIR:-/tmp}
SUP_PID=$RS_RUNTIME_DIR/public-share-supervisor.pid
CHILD_PID=$RS_RUNTIME_DIR/public-share-child.pid
LOG=$RS_RUNTIME_DIR/public-share.log
fail() { echo "RouterShelf: $*" >&2; exit 1; }
for port in "$RS_HTTP_PORT" "$RS_HTTPS_PORT"; do
  case "$port" in ''|*[!0-9]*) fail 'Ports must be integers';; esac
  [ "$port" -ge 1024 ] && [ "$port" -le 65535 ] || fail 'Internal ports must be 1024..65535'
done
[ "$RS_HTTP_PORT" != "$RS_HTTPS_PORT" ] || fail 'HTTP and HTTPS ports must differ'
case "$RS_PUBLIC_ACCESS" in true|false) ;; *) fail 'RS_PUBLIC_ACCESS must be true or false';; esac
case "$RS_DROP_PRIVILEGES" in true|false) ;; *) fail 'RS_DROP_PRIVILEGES must be true or false';; esac
for number in "$RS_CHECK_INTERVAL" "$RS_MIN_YEAR" "$RS_CPU_LIMIT"; do
  case "$number" in ''|*[!0-9]*) fail 'Interval, year and CPU count must be integers';; esac
  [ "$number" -gt 0 ] || fail 'Interval, year and CPU count must be positive'
done
for dir in "$RS_USB_MOUNT" "$RS_ROOT" "$RS_CERT_CACHE" "$RS_BINARY" "$RS_CA_BUNDLE" "$RS_SUPERVISOR_PATH" "$RS_RUNTIME_DIR"; do
  case "$dir" in /*) ;; *) fail 'Deployment paths must be absolute';; esac
  case "$dir" in *' '*|*'..'* ) fail 'Deployment paths must not contain spaces or ..';; esac
done
case "$RS_CERT_CACHE/" in "$RS_ROOT/"*) fail 'Certificate cache must be outside shared root';; esac
export ROUTERSHELF_ENV_FILE="$ENV_FILE"
export RS_ROOT RS_CERT_CACHE RS_DROP_PRIVILEGES
rules() {
  # Block service ports during chain replacement; leave blocks in place on failure.
  old_ports=$(iptables -S INPUT | awk '/-j NB_PUBLIC$/ {for(i=1;i<=NF;i++) if($i=="--dport") print $(i+1)}') || return 1
  ports=$(printf '%s\n%s\n%s\n' "$old_ports" "$RS_HTTP_PORT" "$RS_HTTPS_PORT" | sort -u)
  for port in $ports; do
    [ -n "$port" ] || continue
    iptables -I INPUT 1 -p tcp --dport "$port" -j DROP || return 1
  done
  if ! iptables -S NB_PUBLIC >/dev/null 2>&1; then iptables -N NB_PUBLIC || return 1; fi
  iptables -F NB_PUBLIC || return 1
  if [ "$RS_PUBLIC_ACCESS" = true ]; then
    iptables -A NB_PUBLIC -i "$RS_LAN_INTERFACE" -d "$RS_LAN_IP" -j ACCEPT || return 1
  else
    iptables -A NB_PUBLIC -i "$RS_LAN_INTERFACE" -s "$RS_LAN_SUBNET" -d "$RS_LAN_IP" -j ACCEPT || return 1
  fi
  iptables -A NB_PUBLIC -j DROP || return 1
  for port in $old_ports; do
    iptables -D INPUT -p tcp --dport "$port" -j NB_PUBLIC || return 1
  done
  for port in "$RS_HTTP_PORT" "$RS_HTTPS_PORT"; do
    iptables -I INPUT 1 -p tcp --dport "$port" -j NB_PUBLIC || return 1
    ip6tables -S INPUT | grep -q -- "--dport $port -j DROP" || ip6tables -I INPUT 1 -p tcp --dport "$port" -j DROP || return 1
  done
  for port in $ports; do
    [ -n "$port" ] || continue
    iptables -D INPUT -p tcp --dport "$port" -j DROP || return 1
  done
}
case "${1:-}" in
 --check-config) echo 'RouterShelf configuration valid'; exit 0 ;;
 --firewall) rules; exit $? ;;
 --run)
  child=
  cleanup() { [ -z "$child" ] || kill "$child" 2>/dev/null || :; rm -f "$CHILD_PID" "$SUP_PID"; }
  trap cleanup EXIT
  trap 'exit 0' TERM INT
  while :; do
    if awk -v mount="$RS_USB_MOUNT" '$2==mount {found=1} END{exit !found}' /proc/mounts && [ -d "$RS_ROOT" ] && [ -x "$RS_BINARY" ]; then
      if [ -z "$child" ] || ! kill -0 "$child" 2>/dev/null; then
        [ "$(date +%Y)" -ge "$RS_MIN_YEAR" ] || { sleep "$RS_CHECK_INTERVAL"; continue; }
        rules || { sleep "$RS_CHECK_INTERVAL"; continue; }
        SSL_CERT_FILE="$RS_CA_BUNDLE" GOMEMLIMIT="$RS_MEMORY_LIMIT" GOMAXPROCS="$RS_CPU_LIMIT" "$RS_BINARY" \
          -root "$RS_ROOT" -listen "$RS_LAN_IP:$RS_HTTPS_PORT" -http-listen "$RS_LAN_IP:$RS_HTTP_PORT" \
          -domain "$RS_DOMAIN" -cert-cache "$RS_CERT_CACHE" -drop-privileges="$RS_DROP_PRIVILEGES" &
        child=$!; echo "$child" > "$CHILD_PID"
      fi
    elif [ -n "$child" ]; then
      kill "$child" 2>/dev/null || :; child=; rm -f "$CHILD_PID"
    fi
    sleep "$RS_CHECK_INTERVAL"
  done ;;
 *)
  if ! pidof ntpd >/dev/null; then
    ln -sf /bin/busybox "$RS_RUNTIME_DIR/ntpd"
    "$RS_RUNTIME_DIR/ntpd" -p "$RS_NTP_SERVER1" -p "$RS_NTP_SERVER2"
  fi
  rules || exit 1
  if [ -f "$SUP_PID" ]; then
    pid=$(cat "$SUP_PID")
    case "$pid" in ''|*[!0-9]*) ;; *)
      if [ -r "/proc/$pid/cmdline" ] && tr '\000' ' ' < "/proc/$pid/cmdline" | grep -F "$RS_SUPERVISOR_PATH --run" >/dev/null; then exit 0; fi ;;
    esac
  fi
  nohup "$RS_SUPERVISOR_PATH" --run </dev/null >"$LOG" 2>&1 &
  echo $! > "$SUP_PID" ;;
esac
