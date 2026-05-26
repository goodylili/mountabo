# OPTIONAL — zram compressed-RAM swap (50% of RAM).
log "option: enabling zram compressed swap (50%)"
apt-get install -y zram-tools
if grep -q '^#*PERCENT=' /etc/default/zramswap 2>/dev/null; then sed -i 's/^#*PERCENT=.*/PERCENT=50/' /etc/default/zramswap; else echo 'PERCENT=50' > /etc/default/zramswap; fi
systemctl restart zramswap
