log "option: removing sysctl hardening"
rm -f /etc/sysctl.d/99-mountabo-hardening.conf
sysctl --system >/dev/null
