# OPTIONAL — fail2ban. Appended to the bootstrap only if the operator opts in.
log "option: installing fail2ban (bans IPs after repeated failed logins)"
apt-get install -y fail2ban
systemctl enable --now fail2ban
