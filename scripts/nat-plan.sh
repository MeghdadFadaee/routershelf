#!/bin/sh
# Prints a plan only. It does not connect to or modify the gateway.
set -eu
ENV_FILE=${ROUTERSHELF_ENV_FILE:-.env}
[ -r "$ENV_FILE" ] || { echo "Missing private env: $ENV_FILE" >&2; exit 1; }
. "$ENV_FILE"
: "${RS_GATEWAY_PUBLIC_IP:?}" "${RS_GATEWAY_LAN_IP:?}" "${RS_GATEWAY_LAN_INTERFACE:?}" "${RS_GATEWAY_WAN_INTERFACE:?}"
: "${RS_LAN_IP:?}" "${RS_LAN_SUBNET:?}" "${RS_HTTP_PORT:?}" "${RS_HTTPS_PORT:?}"
: "${RS_PUBLIC_HTTP_PORT:?}" "${RS_PUBLIC_HTTPS_PORT:?}" "${RS_PUBLIC_SSH_PORT:?}" "${RS_SSH_TARGET_PORT:?}"
printf 'RouterShelf gateway NAT plan (manual configuration)\n'
printf 'HTTPS: dstnat dst=%s TCP port=%s -> dst-nat %s:%s\n' "$RS_GATEWAY_PUBLIC_IP" "$RS_PUBLIC_HTTPS_PORT" "$RS_LAN_IP" "$RS_HTTPS_PORT"
printf 'HTTP:  dstnat dst=%s TCP port=%s -> dst-nat %s:%s\n' "$RS_GATEWAY_PUBLIC_IP" "$RS_PUBLIC_HTTP_PORT" "$RS_LAN_IP" "$RS_HTTP_PORT"
printf 'LAN hairpin: srcnat src=%s dst=%s TCP ports=%s,%s out=%s -> src-nat %s\n' "$RS_LAN_SUBNET" "$RS_LAN_IP" "$RS_HTTP_PORT" "$RS_HTTPS_PORT" "$RS_GATEWAY_LAN_INTERFACE" "$RS_GATEWAY_LAN_IP"
printf 'SSH (separate): dstnat in=%s TCP port=%s -> dst-nat %s:%s\n' "$RS_GATEWAY_WAN_INTERFACE" "$RS_PUBLIC_SSH_PORT" "$RS_LAN_IP" "$RS_SSH_TARGET_PORT"
printf 'Preserve existing Internet NAT/filter policy. Public IPv4 changes require DNS and NAT updates.\n'
