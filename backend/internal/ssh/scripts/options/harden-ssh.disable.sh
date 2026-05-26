# OPTIONAL (disable) — un-harden SSH: re-enable root login and password auth.
log "option: un-hardening ssh (re-enable root login + password auth)"
sed -i 's/^#*PermitRootLogin.*/PermitRootLogin yes/; s/^#*PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
printf 'PermitRootLogin yes\nPasswordAuthentication yes\n' > /etc/ssh/sshd_config.d/00-mountabo-ssh.conf
sshd -t
systemctl restart ssh
