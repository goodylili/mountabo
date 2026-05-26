# OPTIONAL (disable) — turn off the UFW firewall.
log "option: disabling ufw firewall"
ufw --force disable || true
