# OPTIONAL — AIDE file-integrity baseline.
log "option: installing aide + building baseline (can take a few minutes)"
apt-get install -y aide aide-common
aideinit || true
if [ -f /var/lib/aide/aide.db.new ]; then mv -f /var/lib/aide/aide.db.new /var/lib/aide/aide.db; fi
