# OPTIONAL — raise file-descriptor limits for high-connection services.
log "option: raising file-descriptor limits (nofile 65535)"
printf '* soft nofile 65535\n* hard nofile 65535\n' > /etc/security/limits.d/99-mountabo.conf
