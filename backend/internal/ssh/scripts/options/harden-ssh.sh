# OPTIONAL — SSH hardening (key-only). Appended only if the operator opts in.
# WARNING: disables root login and password auth. Ensure your own key is
# installed first, or you can be locked out (recover via the provider console).
log "option: hardening ssh (disable root login + password auth, key-only)"
sed -i 's/^#*PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config
sed -i 's/^#*PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
grep -q '^PermitRootLogin no' /etc/ssh/sshd_config || echo 'PermitRootLogin no' >> /etc/ssh/sshd_config
grep -q '^PasswordAuthentication no' /etc/ssh/sshd_config || echo 'PasswordAuthentication no' >> /etc/ssh/sshd_config
sshd -t
systemctl restart ssh
