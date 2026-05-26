# OPTIONAL — chrony NTP time sync.
log "option: installing chrony (accurate time)"
apt-get install -y chrony
systemctl enable --now chrony
