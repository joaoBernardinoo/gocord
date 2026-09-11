#!/usr/bin/env bash
# Gocord / Browser Video Call - interactive installer.
# Generates every config file from a single input: the domain.
set -euo pipefail

ROOT_UID=0
if [[ $EUID -ne $ROOT_UID ]]; then
  echo "Run with sudo: sudo $0" >&2
  exit 1
fi

APP_NAME="ipv6-video-call"
ENV_FILE="/etc/${APP_NAME}.env"
TURN_USER="turnserver"

# ---------- collect input ----------
read -rp "Domain for the app (e.g. call.example.com): " APP_DOMAIN
[[ -n "$APP_DOMAIN" ]] || { echo "Domain is required." >&2; exit 1; }

read -rp "TURN runs on this same host? [Y/n]: " TURN_SAME
TURN_SAME=${TURN_SAME:-y}
if [[ "$TURN_SAME" =~ ^[Yy] ]]; then
  TURN_HOST="$APP_DOMAIN"
else
  read -rp "TURN hostname: " TURN_HOST
fi

# ---------- generated values ----------
TURN_SHARED_SECRET="$(openssl rand -hex 32)"
PUBLIC_IPS="$(ip -brief addr show scope global | awk '{print $3}' | cut -d/ -f1 || true)"

# ---------- requirements ----------
pkg() {
  if command -v apt-get >/dev/null; then apt-get install -y "$@"
  elif command -v dnf >/dev/null; then dnf install -y "$@"
  elif command -v pacman >/dev/null; then pacman -S --noconfirm "$@"
  else echo "Unsupported package manager; install: $*" >&2; exit 1; fi
}

for bin in openssl ip; do
  command -v "$bin" >/dev/null || pkg "$(command -v apt-get >/dev/null && echo openssl || echo iproute2)"
done

# ---------- build or fetch binary ----------
if [[ -d cmd/server ]]; then
  command -v go >/dev/null || echo "Go not found; falling back to release download."
  if command -v go >/dev/null; then
    go build -trimpath -ldflags='-s -w' -o "$APP_NAME" ./cmd/server
  fi
fi
if [[ ! -x "$APP_NAME" ]]; then
  # TODO: replace with actual release asset once goreleaser is set up.
  echo "Download the release binary to ./ and re-run." >&2
  exit 1
fi

# ---------- 1. env file ----------
umask 077
cat > "$ENV_FILE" <<EOF
LISTEN_ADDR=[::1]:8080
PUBLIC_BASE_URL=https://${APP_DOMAIN}
STUN_URLS=stun:${TURN_HOST}:3478
TURN_URLS=turn:${TURN_HOST}:3478?transport=udp,turn:${TURN_HOST}:3478?transport=tcp
TURN_SHARED_SECRET=${TURN_SHARED_SECRET}
MAX_ROOMS=5000
ROOM_CREATE_RATE=0.2
ROOM_CREATE_BURST=5
TRUST_PROXY_HEADERS=true
EOF
echo "Wrote $ENV_FILE"

# ---------- 2. Caddyfile ----------
cat > /etc/caddy/Caddyfile <<EOF
${APP_DOMAIN} {
  reverse_proxy [::1]:8080
}
EOF
systemctl reload caddy || { echo "Install/start Caddy first: apt install caddy" >&2; }

# ---------- 3. coturn ----------
if [[ "$TURN_SAME" =~ ^[Yy] ]]; then
  command -v turnserver >/dev/null || pkg coturn
  cat > /etc/turnserver.conf <<EOF
listening-port=3478
external-ip=${PUBLIC_IPS}
realm=${TURN_HOST}
static-auth-secret=${TURN_SHARED_SECRET}
total-quota=100
bps-capacity=0
stale-nonce=600
no-cli
pidfile=/run/turnserver/turnserver.pid
min-port=49160
max-port=49200
EOF
  systemctl restart coturn
fi

# ---------- 4. firewall ----------
if command -v ufw >/dev/null; then
  ufw allow 3478/udp
  ufw allow 3478/tcp
  ufw allow 49160:49200/udp
elif command -v firewall-cmd >/dev/null; then
  firewall-cmd --permanent --add-port=3478/udp --add-port=3478/tcp \
               --add-port=49160-49200/udp
  firewall-cmd --reload
fi

# ---------- 5. systemd ----------
id -u ipv6call >/dev/null 2>&1 || \
  useradd --system --home /nonexistent --shell /usr/sbin/nologin ipv6call
mkdir -p /opt/ipv6-video-call
install -m 0755 "$APP_NAME" /opt/ipv6-video-call/ipv6-video-call
install -m 0644 deploy/ipv6-video-call.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now ipv6-video-call

# ---------- summary ----------
cat <<EOF

Setup complete.

Secret generated automatically: ${TURN_SHARED_SECRET:0:8}... (full value in $ENV_FILE,
mirrored into /etc/turnserver.conf as static-auth-secret).

Create these DNS records, then open https://${APP_DOMAIN}:
EOF
for ip in $PUBLIC_IPS; do
  if [[ "$ip" == *:* ]]; then echo "  ${APP_DOMAIN}.  AAAA  ${ip}"
  else echo "  ${APP_DOMAIN}.  A     ${ip}"; fi
done
[[ "$TURN_HOST" != "$APP_DOMAIN" ]] && echo "  ${TURN_HOST}.  A/AAAA  (TURN host)"
