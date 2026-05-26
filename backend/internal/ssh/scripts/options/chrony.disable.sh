log "option: disabling chrony"
systemctl disable --now chrony >/dev/null 2>&1 || true
