log "option: reverting sshd to the default port (22)"
rm -f /etc/ssh/sshd_config.d/10-mountabo-ssh-port.conf
sshd -t
systemctl restart ssh
