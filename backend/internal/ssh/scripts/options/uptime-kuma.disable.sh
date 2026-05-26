log "option: removing uptime-kuma container"
docker rm -f uptime-kuma >/dev/null 2>&1 || true
