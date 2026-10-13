#!/usr/bin/env bash
# End-to-end checks for the terminal-only parts of the CLI and the installer.
# Needs no display. GUI/hand-off behaviour is covered by scripts/e2e-gui.sh.
set -uo pipefail
cd "$(dirname "$0")/.."
BIN="${BIN:-$PWD/build/bin/aicw}"
[ -x "$BIN" ] || { echo "build first: scripts/build.sh"; exit 1; }
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
export AICW_DATA_DIR="$T/data"
fail=0
check() { if eval "$2"; then echo "ok   $1"; else echo "FAIL $1"; fail=1; fi; }

check "--version prints version"      '"$BIN" --version | grep -q "^aicw "'
check "--help documents --new"        '"$BIN" --help | grep -q -- "--new"'
check "unknown flag exits 2"          '"$BIN" --bogus 2>/dev/null; [ $? -eq 2 ]'
check "missing path errors, creates nothing" 'mkdir "$T/p"; ! "$BIN" "$T/p/missing" 2>/dev/null && [ ! -e "$T/p/missing" ]'
check "file path rejected non-interactively" 'touch "$T/p/f.txt"; ! "$BIN" "$T/p/f.txt" </dev/null 2>"$T/err" && grep -q "is a file" "$T/err"'
check "--new without path rejected"   '"$BIN" --new 2>/dev/null; [ $? -eq 2 ]'
DOC="$("$BIN" doctor 2>&1 || true)"
check "doctor reports database ok"    'grep -q "\[ok  \] database" <<<"$DOC"'
check "doctor lists adapters"         'grep -q "freebuff" <<<"$DOC"'
check "version/help/doctor never touch the project" '[ -z "$(ls -A "$T/p" | grep -v f.txt)" ]'

# Agent-facing commands (MCP server and `aicw tool`) against a throwaway data dir.
mkdir -p "$T/proj"
check "tool refuses an unknown workspace"  '! AICW_WORKSPACE="$T/proj" "$BIN" tool list_notes 2>"$T/err" && grep -q "not known to AICW" "$T/err"'
check "tool without a workspace explains itself" '! env -u AICW_WORKSPACE "$BIN" tool list_notes 2>"$T/err" && grep -q "no workspace" "$T/err"'
check "mcp refuses an unknown workspace"   '! echo "" | "$BIN" mcp --workspace "$T/proj" 2>"$T/err" && grep -q "not known" "$T/err"'

PKG="$(ls dist/aicw-*-linux-*.tar.gz 2>/dev/null | head -1)"
if [ -n "$PKG" ]; then
  mkdir -p "$T/x" && tar -xzf "$PKG" -C "$T/x" && D="$(echo "$T"/x/aicw-*)"
  check "install into custom prefix"  '(cd "$D" && PREFIX="$T/prefix" ./install.sh >/dev/null) && [ -x "$T/prefix/bin/aicw" ]'
  check "installed aicw runs from any directory" '(cd / && PATH="$T/prefix/bin:$PATH" aicw --version | grep -q "^aicw ")'
  check "doctor finds aicw on PATH"   'OUT="$(cd / && PATH="$T/prefix/bin:$PATH" aicw doctor 2>&1 || true)"; grep -q "\[ok  \] aicw on PATH" <<<"$OUT"'
  check "upgrade (reinstall) works"   'OUT="$(cd "$D" && PREFIX="$T/prefix" ./install.sh)"; grep -q Upgrading <<<"$OUT"'
  check "uninstall keeps user data"   'PREFIX="$T/prefix" "$D/uninstall.sh" >/dev/null && [ ! -e "$T/prefix/bin/aicw" ] && [ -f "$T/data/aicw.db" ]'
else
  echo "skip installer checks (run scripts/package.sh <version> first)"
fi
exit $fail
