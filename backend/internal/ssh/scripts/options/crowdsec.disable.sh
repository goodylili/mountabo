log "option: disabling crowdsec"
systemctl disable --now crowdsec-firewall-bouncer >/dev/null 2>&1 || true
systemctl disable --now crowdsec >/dev/null 2>&1 || true
