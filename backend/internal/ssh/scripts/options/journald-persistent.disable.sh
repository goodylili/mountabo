log "option: reverting journald to default (volatile) storage"
rm -f /etc/systemd/journald.conf.d/00-mountabo.conf
systemctl restart systemd-journald
