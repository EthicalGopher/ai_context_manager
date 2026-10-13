#!/usr/bin/env bash
# Creates dist/aicw-<version>-linux-amd64.tar.gz plus a SHA-256 checksum.
# Usage: scripts/package.sh <version>
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${1:?usage: scripts/package.sh <version>}"
ARCH="$(go env GOARCH)"
NAME="aicw-${VERSION}-linux-${ARCH}"
scripts/build.sh "$VERSION"
rm -rf "dist/$NAME" && mkdir -p "dist/$NAME"
cp build/bin/aicw "dist/$NAME/aicw"
cp build/appicon.png "dist/$NAME/aicw.png"
cp packaging/aicw.desktop scripts/install.sh scripts/uninstall.sh README.md CHANGELOG.md "dist/$NAME/"
tar -C dist -czf "dist/$NAME.tar.gz" "$NAME"
(cd dist && sha256sum "$NAME.tar.gz" > "$NAME.tar.gz.sha256")
echo "wrote dist/$NAME.tar.gz"
