log "option: disabling zram swap"
systemctl disable --now zramswap >/dev/null 2>&1 || true
