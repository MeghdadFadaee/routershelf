#!/bin/sh
# Fixed read-only response. No request content is executed.
SERVICE_DIR=${SERVICE_DIR:-/etc/storage/router-status}
. "$SERVICE_DIR/config.sh"
IFS= read -r -t 5 request || exit 0
case "$request" in
  'GET / HTTP/'*|'GET /status HTTP/'*) ;;
  *) printf 'HTTP/1.1 404 Not Found\r\nConnection: close\r\nContent-Length: 0\r\n\r\n'; exit 0 ;;
esac
while IFS= read -r -t 2 header; do
  [ "$header" = "$(printf '\r')" ] || [ -z "$header" ] && break
done
up=$(awk '{d=int($1/86400);h=int($1%86400/3600);m=int($1%3600/60);printf "%dd %dh %dm",d,h,m}' /proc/uptime)
mem=$(awk '/MemTotal:/{t=$2}/MemFree:/{f=$2}/Buffers:/{b=$2}/^Cached:/{c=$2}END{printf "%.1f / %.1f MB (%.0f%%)",(t-f-b-c)/1024,t/1024,100*(t-f-b-c)/t}' /proc/meminfo)
load=$(awk '{print $1 " / " $2 " / " $3}' /proc/loadavg)
radio='No response'; ping -c 1 -W 1 "$GATEWAY_IP" >/dev/null 2>&1 && radio='Responding'
internet='No response'; ping -c 1 -W 2 "$PROBE_IP" >/dev/null 2>&1 && internet='Probe succeeded'
printf 'HTTP/1.1 200 OK\r\nContent-Type: text/html; charset=utf-8\r\nCache-Control: no-store\r\nX-Content-Type-Options: nosniff\r\nContent-Security-Policy: default-src '\''none'\''; style-src '\''unsafe-inline'\''; frame-ancestors '\''none'\''\r\nConnection: close\r\n\r\n'
cat <<EOF
<!doctype html>
<html lang="en"><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="refresh" content="30">
<title>Router status</title>
<style>
body{font:17px system-ui,sans-serif;background:#edf3f8;color:#183248;margin:0;padding:32px 16px}
main{max-width:700px;margin:auto}h1{font-size:29px}
p,footer{color:#51687b;line-height:1.7}
.card{background:white;border-radius:16px;padding:20px;margin:14px 0;display:flex;justify-content:space-between;gap:20px;flex-wrap:wrap}
.value{font-weight:600}footer{font-size:14px}
</style></head><body><main>
<h1>Router status</h1>
<p>Local network dashboard. Refreshes every 30 seconds.</p>
<div class="card"><span>Uptime</span><span class="value">$up</span></div>
<div class="card"><span>Memory used, excluding cache</span><span class="value">$mem</span></div>
<div class="card"><span>Load average: 1 / 5 / 15 minutes</span><span class="value">$load</span></div>
<div class="card"><span>Gateway probe</span><span class="value">$radio</span></div>
<div class="card"><span>Internet IP probe</span><span class="value">$internet</span></div>
<footer>Probes run on the router when this page is requested. A failed ICMP probe does not prove that Internet access is down. Load average is not CPU utilization.</footer>
</main></body></html>
EOF
