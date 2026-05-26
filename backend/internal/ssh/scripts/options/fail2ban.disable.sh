# OPTIONAL (disable) — stop and disable fail2ban.
log "option: disabling fail2ban"
systemctl disable --now fail2ban || true
