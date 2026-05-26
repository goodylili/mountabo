# OPTIONAL — Netdata real-time monitoring, bound to localhost:19999.
log "option: installing netdata (bound to 127.0.0.1:19999)"
wget -qO /tmp/netdata-kickstart.sh https://get.netdata.cloud/kickstart.sh
sh /tmp/netdata-kickstart.sh --stable-channel --disable-telemetry --non-interactive
mkdir -p /etc/netdata
printf '[web]\n    bind to = 127.0.0.1\n' > /etc/netdata/netdata.conf
systemctl restart netdata || true
