# OPTIONAL — UFW firewall. Appended to the bootstrap only if the operator opts in.
log "option: configuring ufw firewall (deny inbound except ssh/http/https)"
apt-get install -y ufw
ufw default deny incoming
ufw default allow outgoing
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable
