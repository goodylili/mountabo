# OPTIONAL (disable) — turn off automatic security updates.
log "option: disabling automatic security updates"
printf 'APT::Periodic::Update-Package-Lists "0";\nAPT::Periodic::Unattended-Upgrade "0";\n' > /etc/apt/apt.conf.d/20auto-upgrades
