# OPTIONAL — rkhunter rootkit scanner (on-demand).
log "option: installing rkhunter"
apt-get install -y rkhunter
rkhunter --propupd >/dev/null 2>&1 || true
