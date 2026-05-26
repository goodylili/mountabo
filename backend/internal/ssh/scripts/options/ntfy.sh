# OPTIONAL — ntfy push notifications (container, 127.0.0.1:8080).
log "option: starting ntfy (127.0.0.1:8080)"
docker rm -f ntfy >/dev/null 2>&1 || true
docker run -d --restart=unless-stopped -p 127.0.0.1:8080:80 -v ntfy-cache:/var/cache/ntfy --name ntfy binwiederhier/ntfy:latest serve
