# OPTIONAL — Uptime Kuma status monitor (container, 127.0.0.1:3001).
log "option: starting uptime-kuma (127.0.0.1:3001)"
docker rm -f uptime-kuma >/dev/null 2>&1 || true
docker run -d --restart=unless-stopped -p 127.0.0.1:3001:3001 -v uptime-kuma:/app/data --name uptime-kuma louislam/uptime-kuma:1
