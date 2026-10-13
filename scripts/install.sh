#!/usr/bin/env bash
# Installs aicw for the current user (no root needed) or into a custom prefix.
#   ./install.sh                 -> ~/.local  (binary in ~/.local/bin/aicw)
#   PREFIX=/opt/aicw ./install.sh
# Upgrades replace only the program files. Saved workspaces, sessions, context
# files and project skills live elsewhere and are never touched.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
SRC="$HERE/aicw"; [ -x "$SRC" ] || SRC="$HERE/../build/bin/aicw"
[ -x "$SRC" ] || { echo "aicw binary not found next to this script; run scripts/build.sh first" >&2; exit 1; }
PREFIX="${PREFIX:-$HOME/.local}"
BIN="$PREFIX/bin"; APPS="$PREFIX/share/applications"; ICONS="$PREFIX/share/icons/hicolor/512x512/apps"
mkdir -p "$BIN" "$APPS" "$ICONS"
if [ -e "$BIN/aicw" ]; then echo "Upgrading existing install: $("$BIN/aicw" --version 2>/dev/null || echo unknown) -> $("$SRC" --version)"; fi
# Write to a temp name then rename, so a running instance is not corrupted.
install -m 0755 "$SRC" "$BIN/aicw.new" && mv -f "$BIN/aicw.new" "$BIN/aicw"
ICON="$HERE/aicw.png"; [ -f "$ICON" ] || ICON="$HERE/../build/appicon.png"
[ -f "$ICON" ] && install -m 0644 "$ICON" "$ICONS/aicw.png"
DESK="$HERE/aicw.desktop"; [ -f "$DESK" ] || DESK="$HERE/../packaging/aicw.desktop"
sed "s|^Exec=aicw|Exec=$BIN/aicw|" "$DESK" > "$APPS/aicw.desktop"
echo "Installed $BIN/aicw"
case ":$PATH:" in *":$BIN:"*) ;; *) echo "NOTE: $BIN is not on your PATH. Add this to your shell profile:"; echo "  export PATH=\"$BIN:\$PATH\"";; esac
echo "Verify with: aicw doctor"
