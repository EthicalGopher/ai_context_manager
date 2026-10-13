#!/usr/bin/env bash
# Starts the real app, then verifies a second `aicw <path>` is handed to the
# running instance (exit 0, no duplicate process, workspace recorded, no session
# created). Requires a display (X11/Wayland); skips otherwise.
set -uo pipefail
cd "$(dirname "$0")/.."
BIN="${BIN:-$PWD/build/bin/aicw}"
[ -n "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ] || { echo "skip: no display"; exit 0; }
T="$(mktemp -d)"; export AICW_DATA_DIR="$T/data"
mkdir -p "$T/proj one" "$T/proj two"
"$BIN" "$T/proj one" >"$T/app.log" 2>&1 & APP=$!
cleanup() { kill "$APP" 2>/dev/null; wait "$APP" 2>/dev/null; rm -rf "$T"; }; trap cleanup EXIT
fail=0; check() { if eval "$2"; then echo "ok   $1"; else echo "FAIL $1"; fail=1; fi; }
for _ in $(seq 1 50); do [ -S "$T/data/aicw.sock" ] && break; sleep 0.2; done
check "app started and is listening" '[ -S "$T/data/aicw.sock" ] && kill -0 "$APP"'
(cd "$T/proj one" && "$BIN" . >"$T/second.log" 2>&1); RC=$?
check "second 'aicw .' exits 0"        '[ "$RC" -eq 0 ]'
(cd "$T" && "$BIN" "proj two" >/dev/null 2>&1); RC2=$?
check "second path with a space exits 0" '[ "$RC2" -eq 0 ]'
sleep 1
OUT="$(AICW_WORKSPACE="$T/proj one" "$BIN" tool write_page '{"title":"Character","body":"# Hi\n\nFrom an agent."}' 2>&1)"
check "agent tool writes a page into the known workspace" 'grep -q "\"ok\": true" <<<"$OUT"'
MCP="$(printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_pages","arguments":{}}}' | "$BIN" mcp --workspace "$T/proj one" 2>&1)"
check "MCP server lists that page" 'grep -q "Character" <<<"$MCP"'
check "still exactly one app process"  '[ "$(pgrep -c -x "$(basename "$BIN")")" -eq 1 ]'
ROWS="$(python3 - "$T/data/aicw.db" <<'PY'
import sqlite3,sys
c=sqlite3.connect(sys.argv[1])
print(len(c.execute("select 1 from workspaces").fetchall()), len(c.execute("select 1 from sessions").fetchall()))
PY
)"
check "both workspaces recorded, zero sessions created" '[ "$ROWS" = "2 0" ]'
exit $fail
