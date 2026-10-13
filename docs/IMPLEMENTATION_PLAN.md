# Implementation plan and progress

A milestone is ticked only when its acceptance checks pass **on this machine** (Linux). Windows/macOS items are not claimed.

## Decisions

- Wails **v2.13.0** (Go 1.27 toolchain here; `go.mod` says `go 1.26`). React 19, Vite 7, Tailwind 3, React Router (HashRouter), Zustand, `react-icons`.
- SQLite via `modernc.org/sqlite` (pure Go, no CGO beyond Wails/WebKit).
- No embedded terminal: agents open in the user's terminal emulator (smaller, safer, no PTY code to maintain).
- Wails CLI `build`/`dev` fail on Go 1.27 (`package "errors" without types`), so `scripts/build.sh` calls `go build` with Wails' tags; `wails dev` hot reload is unverified.

## Milestones

- [x] **1 Audit & skeleton** — reference analysis, Wails + React + Tailwind scaffold, app launches (`scripts/e2e-gui.sh`).
- [x] **2 CLI & workspace opening** — `internal/cli`, `internal/ipc`; paths with spaces, `~`, relative, invalid, file-vs-dir; hand-off to running instance (exit 0).
- [x] **3 Scanner** — `internal/scanner`: discovery rules, ignores, limits, symlink containment, secret/injection heuristics, Git status.
- [x] **4 Context & Markdown** — `internal/contextbuilder`, `internal/markdown`: budget, export, diff, backup/restore/audit, archive/undo.
- [x] **5 Agent adapters** — `internal/agents`, `internal/process`: six adapters, custom profiles, detection, shell-free launch. *Real end-to-end launch of a CLI was not executed* (it would start a paid session); argv construction, detection and terminal command building are unit-tested.
- [x] **6 Skills & handoff** — `internal/skills`, `internal/history`, Skills and Session pages.
- [~] **7 Polish & release** — tests, docs, tarball + install/upgrade/uninstall verified locally. **Open:** Windows/macOS builds, AppImage, signed artifacts, browser E2E, CI workflows never run on GitHub, screenshots.

## Known gaps / next

1. Detect when a launched agent's terminal closes (status is currently set by the user or by PID liveness of the launcher, which is unreliable for emulators that daemonize).
2. Optional lifecycle hooks (never assumed) for per-agent session-end reminders.
3. Windows/macOS terminal launch and packaging.
4. Playwright-style UI E2E against `wails dev`.

## 0.2 additions

- [x] Chat replies, queue/stop, edits opt-in (`service/chat.go`, `agents.BuildChat`)
- [x] PTY terminal (`internal/term`, `TerminalPane`)
- [x] Tools, MCP, `aicw tool` (`internal/tools`, `internal/mcp`)
- [x] Always-on skills, notes, managed instruction block (`internal/rules`)
- [x] Pages + Save to page + file browser
- [x] Voice recording + local transcription + offline setup
- [ ] Verify against real agents (Claude/Kilo/AGY chat turn, Claude MCP) — needs a paid call; left to the user
- [ ] Streaming replies (currently the reply appears when the turn finishes)
- [ ] Codex/Gemini chat adapters once verified on an installed CLI
- [ ] Windows/macOS PTY and packaging
