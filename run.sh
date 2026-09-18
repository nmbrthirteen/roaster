#!/usr/bin/env bash
# Build and run the roaster on macOS or Linux.
set -euo pipefail
cd "$(dirname "$0")"

printf '\n  Roaster\n  -------\n\n'

if ! command -v go >/dev/null 2>&1; then
  echo "  Go is not installed."
  if command -v brew >/dev/null 2>&1; then
    echo "  Installing with Homebrew..."
    brew install go
  else
    echo "  Install it from https://go.dev/dl and run this again."
    exit 1
  fi
fi

if [ -d .git ] && git diff --quiet 2>/dev/null; then
  echo "  Pulling latest..."
  git pull --quiet || true
fi

# Integral CF is a Fontspring demo and is not in the repo. Without it the
# headline falls back to a system face and looks wrong.
SRC="../lifeatupgaming-front/public/fonts/integralcf"
if [ ! -f internal/ui/fonts/integral-heavy.woff2 ] && [ -d "$SRC" ]; then
  for w in regular bold heavy; do
    cp "$SRC/Fontspring-DEMO-integralcf-$w.woff2" "internal/ui/fonts/integral-$w.woff2"
  done
  echo "  Headline font copied from the lifeat frontend."
fi

[ -f roaster.json ] || cp roaster.example.json roaster.json

echo "  Building..."
go build -o roaster ./cmd/roaster

PORT=$(sed -n 's/.*"addr": *":\([0-9]*\)".*/\1/p' roaster.json)
PORT=${PORT:-3000}
echo "  Opening http://localhost:$PORT/kiosk"
( sleep 2; open "http://localhost:$PORT/kiosk" 2>/dev/null || xdg-open "http://localhost:$PORT/kiosk" 2>/dev/null ) &

printf '\n'
exec ./roaster
