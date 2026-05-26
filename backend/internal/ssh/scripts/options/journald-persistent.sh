# OPTIONAL — persistent journald logs (survive reboots, capped at 2G).
log "option: enabling persistent journald logs (max 2G)"
mkdir -p /etc/systemd/journald.conf.d
printf '[Journal]\nStorage=persistent\nSystemMaxUse=2G\n' > /etc/systemd/journald.conf.d/00-mountabo.conf
systemctl restart systemd-journald
