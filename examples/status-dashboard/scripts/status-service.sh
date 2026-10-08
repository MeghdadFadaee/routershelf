#!/bin/sh
set -e
SERVICE_DIR=${SERVICE_DIR:-/etc/storage/router-status}
. "$SERVICE_DIR/config.sh"
rules() {
  if ! iptables -S RS_STATUS >/dev/null 2>&1; then
    iptables -N RS_STATUS
    iptables -A RS_STATUS -j DROP
    iptables -I RS_STATUS 1 -i "$LAN_INTERFACE" -s "$LAN_SUBNET" -d "$LAN_IP" -j ACCEPT
  fi
  # Refuse to silently reuse a chain configured for another LAN.
  iptables -S RS_STATUS | grep -F -- "-s $LAN_SUBNET -d $LAN_IP/32 -i $LAN_INTERFACE -j ACCEPT" >/dev/null || {
    echo "Review RS_STATUS: expected LAN allow rule not found" >&2
    return 1
  }
  iptables -S RS_STATUS | grep -q -- '-A RS_STATUS -j DROP' || return 1
  ip6tables -S INPUT | grep -q -- "--dport $PORT -j DROP" || ip6tables -I INPUT 1 -p tcp --dport "$PORT" -j DROP
  iptables -S INPUT | grep -q -- "--dport $PORT -j RS_STATUS" || iptables -I INPUT 1 -p tcp --dport "$PORT" -j RS_STATUS
}
rules
[ "$1" = '--firewall' ] && exit 0
if [ -f /tmp/router-status.pid ]; then
  pid=$(cat /tmp/router-status.pid)
  case "$pid" in ''|*[!0-9]*) ;; *) kill -0 "$pid" 2>/dev/null && exit 0 ;; esac
fi
export SERVICE_DIR
nohup nc -ll -p "$PORT" -e "$SERVICE_DIR/status-response.sh" </dev/null >/tmp/router-status.log 2>&1 &
echo $! >/tmp/router-status.pid
