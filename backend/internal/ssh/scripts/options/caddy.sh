# OPTIONAL — Caddy reverse proxy + automatic HTTPS. {{.domain}} -> localhost:{{.upstream}}.
log "option: installing caddy, serving {{.domain}} -> localhost:{{.upstream}}"
apt-get install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --batch --yes --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' > /etc/apt/sources.list.d/caddy-stable.list
apt-get update -y
apt-get install -y caddy
printf '%s {\n    reverse_proxy localhost:%s\n}\n' '{{.domain}}' '{{.upstream}}' > /etc/caddy/Caddyfile
systemctl reload caddy || systemctl restart caddy
