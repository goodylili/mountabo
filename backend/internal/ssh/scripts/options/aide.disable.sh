log "option: removing aide"
apt-get purge -y aide aide-common >/dev/null 2>&1 || true
rm -f /var/lib/aide/aide.db /var/lib/aide/aide.db.new
