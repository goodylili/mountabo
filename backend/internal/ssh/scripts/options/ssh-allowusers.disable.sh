log "option: removing ssh AllowUsers restriction"
rm -f /etc/ssh/sshd_config.d/10-mountabo-allowusers.conf
sshd -t
systemctl restart ssh
