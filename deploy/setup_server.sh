#!/usr/bin/env bash
set -euo pipefail

echo "=== 1. Updating packages and installing prerequisites ==="
sudo dnf install -y epel-release dnf-plugins-core
sudo dnf install -y golang git coturn || sudo dnf install -y golang git

echo "=== 2. Installing Caddy ==="
if ! command -v caddy &>/dev/null; then
  sudo dnf copr enable -y @caddy/caddy || true
  sudo dnf install -y caddy || true
fi

echo "=== 3. Creating application user and directories ==="
sudo useradd -r -s /sbin/nologin ipv6call || true
sudo mkdir -p /opt/ipv6-video-call
sudo chown -R ipv6call:ipv6call /opt/ipv6-video-call

echo "=== 4. Building Go Application ==="
APP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$APP_DIR"
go mod download
go build -trimpath -ldflags='-s -w' -o /opt/ipv6-video-call/ipv6-video-call ./cmd/server
sudo chown ipv6call:ipv6call /opt/ipv6-video-call/ipv6-video-call
sudo chmod 755 /opt/ipv6-video-call/ipv6-video-call

echo "=== 5. Setting up Environment File ==="
if [ ! -f /etc/ipv6-video-call.env ]; then
  SHARED_SECRET=$(openssl rand -hex 24)
  sudo tee /etc/ipv6-video-call.env > /dev/null <<ENVCONF
LISTEN_ADDR=[::]:8080
PUBLIC_BASE_URL=https://call.example.com
ROOM_TTL=2h
EMPTY_ROOM_GRACE=45s
SHUTDOWN_TIMEOUT=10s
DEV_DIAGNOSTICS=true
STUN_URLS=stun:stun.l.google.com:19302
TURN_URLS=
TURN_SHARED_SECRET=${SHARED_SECRET}
TURN_CREDENTIAL_TTL=1h
ICE_SERVERS_JSON=[]
ENVCONF
  sudo chmod 600 /etc/ipv6-video-call.env
  sudo chown ipv6call:ipv6call /etc/ipv6-video-call.env
fi

echo "=== 6. Installing Systemd Service ==="
sudo cp "$APP_DIR/deploy/ipv6-video-call.service" /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now ipv6-video-call

echo "=== 7. Configuring Firewall (firewalld) ==="
if systemctl is-active --quiet firewalld; then
  sudo firewall-cmd --permanent --add-port=8080/tcp || true
  sudo firewall-cmd --permanent --add-service=http || true
  sudo firewall-cmd --permanent --add-service=https || true
  sudo firewall-cmd --permanent --add-port=3478/tcp || true
  sudo firewall-cmd --permanent --add-port=3478/udp || true
  sudo firewall-cmd --permanent --add-port=49160-49200/udp || true
  sudo firewall-cmd --reload || true
fi

echo "=== Deployment Completed Successfully! ==="
sudo systemctl status ipv6-video-call --no-pager
