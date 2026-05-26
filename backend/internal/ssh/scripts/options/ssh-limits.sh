# OPTIONAL — limit SSH auth attempts per connection.
log "option: limiting ssh auth attempts (MaxAuthTries 3, LoginGraceTime 30)"
printf 'MaxAuthTries 3\nLoginGraceTime 30\n' > /etc/ssh/sshd_config.d/10-mountabo-ssh-limits.conf
sshd -t
systemctl restart ssh
