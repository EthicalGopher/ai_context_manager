#!/usr/bin/env bash
# Builds the frontend and the aicw binary. Usage: scripts/build.sh [version]
# Uses `go build` with Wails' production tags directly (the wails CLI v2.13
# currently fails its analysis step on Go 1.27; the result is the same binary).
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${1:-$(git describe --tags --always 2>/dev/null || echo 0.1.0-dev)}"
TAGS="desktop,production"
# webkit2gtk-4.1 is what current distros ship; fall back to 4.0 if only that exists.
if pkg-config --exists webkit2gtk-4.1 2>/dev/null; then TAGS="$TAGS,webkit2_41"; fi
(cd frontend && npm ci --silent && npm run build --silent)
mkdir -p build/bin
go build -tags "$TAGS" -ldflags "-s -w -X main.Version=$VERSION" -o build/bin/aicw .
echo "built build/bin/aicw ($VERSION)"
