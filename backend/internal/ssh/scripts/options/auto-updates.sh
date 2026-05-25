# OPTIONAL — automatic security updates. Appended only if the operator opts in.
log "option: enabling unattended-upgrades (automatic security patches)"
apt-get install -y unattended-upgrades
echo 'unattended-upgrades unattended-upgrades/enable_auto_updates boolean true' | debconf-set-selections
dpkg-reconfigure -f noninteractive unattended-upgrades
