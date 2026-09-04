#!/usr/bin/env bash
set -euo pipefail

HOST="${1:?Usage: $0 <REMOTE_HOST> [USER]}"
USER="${2:-ubuntu}"
REMOTE_DEST="/home/$USER/ipv6-video-call"

echo "=== Syncing project files to $USER@$HOST:$REMOTE_DEST ==="
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

rsync -avz \
  --exclude='.git' \
  --exclude='bin' \
  --exclude='tmp' \
  --exclude='server' \
  --exclude='ipv6-video-call' \
  --exclude='vapid_keys.json' \
  --exclude='*.snapshot' \
  --exclude='.env' \
  "$PROJECT_ROOT/" "$USER@$HOST:$REMOTE_DEST/"

echo "=== Sync complete! ==="
echo "=== Rebuilding Docker containers on remote host ==="
ssh "$USER@$HOST" "cd $REMOTE_DEST && docker compose up -d --build"

echo "=== Deployment complete! ==="
