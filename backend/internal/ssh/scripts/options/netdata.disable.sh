log "option: disabling netdata"
systemctl disable --now netdata >/dev/null 2>&1 || true
