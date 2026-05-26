# OPTIONAL — SSH hardening (key-only). Appended only if the operator opts in.
# WARNING: disables root login and password auth. Ensure your own key is
# installed first, or you can be locked out (recover via the provider console).
# Uses a 00- drop-in so it wins over cloud-init / other drop-ins, and clears the
# root-password drop-in mountabo may have written earlier.
log "option: hardening ssh (disable root login + password auth, key-only)"
sed -i 's/^#*PermitRootLogin.*/PermitRootLogin no/; s/^#*PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
rm -f /etc/ssh/sshd_config.d/00-mountabo-rootpw.conf
printf 'PermitRootLogin no\nPasswordAuthentication no\n' > /etc/ssh/sshd_config.d/00-mountabo-ssh.conf
sshd -t
systemctl restart ssh
