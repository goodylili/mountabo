# OPTIONAL — sysctl kernel hardening (SYN cookies, restricted dmesg, rp_filter).
log "option: applying sysctl hardening"
printf 'net.ipv4.tcp_syncookies=1\nkernel.dmesg_restrict=1\nnet.ipv4.conf.all.rp_filter=1\nnet.ipv4.conf.all.accept_redirects=0\nnet.ipv6.conf.all.accept_redirects=0\nnet.ipv4.conf.all.send_redirects=0\n' > /etc/sysctl.d/99-mountabo-hardening.conf
sysctl --system >/dev/null
