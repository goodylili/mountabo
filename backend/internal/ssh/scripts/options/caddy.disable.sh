log "option: disabling caddy"
systemctl disable --now caddy >/dev/null 2>&1 || true
