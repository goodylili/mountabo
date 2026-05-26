# OPTIONAL — move sshd off port 22. {{.port}} is supplied by the operator.
log "option: moving sshd to port {{.port}}"
printf 'Port {{.port}}\n' > /etc/ssh/sshd_config.d/10-mountabo-ssh-port.conf
command -v ufw >/dev/null 2>&1 && ufw allow {{.port}}/tcp || true
sshd -t
systemctl restart ssh
