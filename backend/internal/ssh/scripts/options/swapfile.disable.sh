log "option: removing swap file"
swapoff /swapfile 2>/dev/null || true
sed -i '\#^/swapfile #d' /etc/fstab
rm -f /swapfile
