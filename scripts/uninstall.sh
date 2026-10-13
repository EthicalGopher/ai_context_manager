#!/usr/bin/env bash
# Removes the program files. Your data is kept unless you pass --purge-data.
#   PREFIX=... ./uninstall.sh [--purge-data]
set -euo pipefail
PREFIX="${PREFIX:-$HOME/.local}"
rm -f "$PREFIX/bin/aicw" "$PREFIX/share/applications/aicw.desktop" "$PREFIX/share/icons/hicolor/512x512/apps/aicw.png"
echo "Removed program files from $PREFIX."
DATA="${AICW_DATA_DIR:-${XDG_DATA_HOME:-$HOME/.local/share}/aicw}"
if [ "${1:-}" = "--purge-data" ]; then
  read -r -p "Delete ALL AICW data (workspaces, sessions, generated context, backups) in $DATA? Type 'delete' to confirm: " ans
  [ "$ans" = "delete" ] && rm -rf "$DATA" && echo "Deleted $DATA" || echo "Kept $DATA"
else
  echo "Kept your data in $DATA (re-run with --purge-data to remove it). Project files and .aicw/ folders are never touched."
fi
