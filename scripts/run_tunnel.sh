#!/usr/bin/env bash
# Run miner + a free Cloudflare quick tunnel (temporary https://*.trycloudflare.com).
# Gives phone browsers a secure context so getUserMedia / in-page camera works.
#
# Prerequisites: MINER_PIN (env or .env), bin/miner (make build), cloudflared on PATH.
# No Cloudflare account required for quick tunnels.
#
# Security: the printed URL is reachable on the public Internet until you stop
# this script. Only the PIN gate protects the app — stop the tunnel when done.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# Early PIN check (miner also loads .env itself).
# MINER_TUNNEL_FORCE_NO_ENV=1: test helper — ignore .env (still honor MINER_PIN).
if [[ -z "${MINER_PIN:-}" ]]; then
  if [[ "${MINER_TUNNEL_FORCE_NO_ENV:-}" == "1" ]] || \
     [[ ! -f .env ]] || ! grep -qE '^[[:space:]]*(export[[:space:]]+)?MINER_PIN=.+' .env; then
    echo "MINER_PIN is required (add to .env or export MINER_PIN=...)" >&2
    echo "  cp .env.example .env   # then edit MINER_PIN" >&2
    exit 1
  fi
fi

# Prefer loopback when tunneling so miner is not also open on the LAN unless
# the user explicitly sets MINER_ADDR (e.g. :8080 for dual LAN+tunnel use).
export MINER_ADDR="${MINER_ADDR:-127.0.0.1:8080}"

case "$MINER_ADDR" in
  :*) port="${MINER_ADDR#:}" ;;
  *:[0-9]*) port="${MINER_ADDR##*:}" ;;
  *)
    echo "MINER_ADDR=$MINER_ADDR: expected host:port or :port" >&2
    exit 1
    ;;
esac
if [[ ! "$port" =~ ^[0-9]+$ ]]; then
  echo "MINER_ADDR=$MINER_ADDR: could not parse port" >&2
  exit 1
fi
LOCAL_URL="http://127.0.0.1:${port}"

# MINER_TUNNEL_CHECK_ONLY=1: validate PIN + addr only (unit tests; no processes).
if [[ "${MINER_TUNNEL_CHECK_ONLY:-}" == "1" ]]; then
  echo "tunnel check-only ok target=${LOCAL_URL}"
  exit 0
fi

if ! command -v cloudflared >/dev/null 2>&1; then
  cat >&2 <<'EOF'
cloudflared not found on PATH.

Install:
  macOS:  brew install cloudflared
  other:  https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/
EOF
  exit 1
fi

BIN="${ROOT}/bin/miner"
if [[ ! -x "$BIN" ]]; then
  echo "bin/miner missing or not executable. Run: make build" >&2
  exit 1
fi

miner_pid=""
cleanup() {
  if [[ -n "${miner_pid:-}" ]] && kill -0 "$miner_pid" 2>/dev/null; then
    kill "$miner_pid" 2>/dev/null || true
    wait "$miner_pid" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

echo "Starting miner on ${MINER_ADDR} (tunnel target ${LOCAL_URL})"
"$BIN" &
miner_pid=$!

ready=0
for _ in $(seq 1 75); do
  if ! kill -0 "$miner_pid" 2>/dev/null; then
    echo "miner exited before becoming ready" >&2
    exit 1
  fi
  if command -v curl >/dev/null 2>&1; then
    if curl -sf -o /dev/null --max-time 1 "${LOCAL_URL}/" 2>/dev/null; then
      ready=1
      break
    fi
  else
    # Bash /dev/tcp readiness (no HTTP check).
    if (echo >/dev/tcp/127.0.0.1/"$port") 2>/dev/null; then
      ready=1
      break
    fi
  fi
  sleep 0.2
done
if [[ "$ready" -ne 1 ]]; then
  echo "timed out waiting for miner at ${LOCAL_URL}" >&2
  exit 1
fi

cat <<EOF

Cloudflare quick tunnel (free, temporary HTTPS)
  - Open the https://*.trycloudflare.com URL printed below on your phone
  - Secure context → in-page camera should work in Safari
  - URL changes each run; Ctrl+C stops miner + tunnel
  - Public Internet can reach the PIN page until you stop — use a strong PIN

EOF

# Foreground so logs (including the trycloudflare.com URL) stay visible.
# --no-autoupdate avoids surprise restarts mid-session.
cloudflared tunnel --no-autoupdate --url "${LOCAL_URL}"
