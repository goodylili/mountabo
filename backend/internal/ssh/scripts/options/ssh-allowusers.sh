# OPTIONAL — restrict SSH logins to specific users (mountabo + root).
log "option: restricting ssh logins to mountabo and root"
printf 'AllowUsers mountabo root\n' > /etc/ssh/sshd_config.d/10-mountabo-allowusers.conf
sshd -t
systemctl restart ssh
