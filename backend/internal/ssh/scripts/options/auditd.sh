# OPTIONAL — auditd kernel-level audit logging.
log "option: installing auditd"
apt-get install -y auditd audispd-plugins
systemctl enable --now auditd
