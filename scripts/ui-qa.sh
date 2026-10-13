#!/usr/bin/env bash
# Headless-Chrome screenshots of the real frontend with a mocked backend.
# Usage: scripts/ui-qa.sh <outdir> [route ...]   (routes like "#/p/2" "#/c/1")
set -euo pipefail
cd "$(dirname "$0")/.."
OUT="${1:?outdir}"; shift
ROUTES=("${@:-#/p/2}")
STAGE="$(mktemp -d)"; trap 'kill $SRV 2>/dev/null; rm -rf "$STAGE"' EXIT
cp -r frontend/dist/. "$STAGE/"
cp scripts/qa/mock-bridge.js "$STAGE/mock-bridge.js"
sed -i 's#<script type="module"#<script src="/mock-bridge.js"></script><script type="module"#' "$STAGE/index.html"
MEDIA="${QA_MEDIA:-/tmp/claude-1000/demo/media}"
python3 scripts/qa/serve.py "$STAGE" "$MEDIA" 8765 & SRV=$!
sleep 1; mkdir -p "$OUT"
i=0
for r in "${ROUTES[@]}"; do
  i=$((i+1))
  google-chrome-stable --headless=new --no-sandbox --disable-gpu --hide-scrollbars --window-size=${QA_SIZE:-1500,950} \
    --virtual-time-budget=${QA_BUDGET:-6000} --screenshot="$OUT/qa-$i.png" "http://127.0.0.1:8765/$r" >/dev/null 2>&1 || true
  echo "$OUT/qa-$i.png  <- $r"
done
