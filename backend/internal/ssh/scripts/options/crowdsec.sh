# OPTIONAL — CrowdSec: community-shared brute-force blocking.
log "option: installing crowdsec + firewall bouncer"
curl -s https://install.crowdsec.net | sh
apt-get install -y crowdsec crowdsec-firewall-bouncer-iptables
systemctl enable --now crowdsec
