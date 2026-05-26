log "option: disabling auditd"
systemctl disable --now auditd >/dev/null 2>&1 || true
