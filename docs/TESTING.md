# Testing

```bash
go test ./...                       # Go tests
(cd frontend && npm test)           # Vitest tests (jsdom, mocked bindings)
scripts/build.sh 0.1.0 && scripts/package.sh 0.1.0
scripts/e2e-cli.sh                  # terminal-only CLI + installer flow (no display)
scripts/e2e-gui.sh                  # real app + IPC hand-off + agent tools (needs a display)
scripts/ui-qa.sh <dir> "#/p/2" "#/c/1?term=1"   # headless-Chrome screenshots with a mocked backend
```

All of the above passed on the development machine at the time of writing. Rerun them; do not rely on this sentence.

## What is covered

| Requirement | Where |
| --- | --- |
| `.`, absolute, relative, spaces, `~`, invalid, file paths | `safety`, `cli` tests; `e2e-cli.sh` |
| New workspace via CLI / delivery to running instance | `ipc` tests; `e2e-gui.sh` (exit 0, one process, zero sessions) |
| Ignore dirs, `.gitignore`, limits | `scanner` tests |
| Symlink escape and traversal | `safety`, `scanner`, `skills`, `markdown`, `service` tests |
| Secret detection and exclusion; injection warnings | `safety`, `scanner`, `contextbuilder`, `service` (incl. a regression for a README leak) |
| Deterministic package, budget enforcement | `contextbuilder` tests |
| Diff, overwrite protection, backup/restore, audit, move/undo | `markdown`, `service` tests |
| Agent detection, argv handling, shell metacharacters, missing CLI | `agents`, `process`, `service` tests |
| Skills discovery without execution | `skills` test (a skill that would `touch` a marker file) |
| `aicw .` restores workspace, no new session | `workspace`, `service`, `storage` tests; concurrency test (30 writers) |
| Frontend loading/error/confirmation states, resume-first panel | `frontend/src/**/*.test.tsx` |

Added in 0.2: chat turns with fake CLIs (reply, session continuation, edits opt-in, failure, missing CLI, queue, stop), PTY (echo, exit code, argv safety, resize, kill), tools and MCP (limits, isolation, no forged user messages, media refusal), pages (versions, case-insensitive titles, embedded local media), instruction-block install, voice (recorder, transcript cleanup, engine detection), file browser (traversal, symlink escape, secret flags), modal focus regression, composer/chat/pages UI.

## Not covered (be aware)

- **No real agent was ever called** (cost). Chat and terminal launches were exercised with fake CLIs; Claude's MCP hookup is untested against a real session.
- The xterm view was only rendered with canned output; interactive typing in the real window was not driven.
- Voice accuracy was only sampled on synthetic speech; a live mic dictation round trip was not run.

- No browser/WebView-driven UI E2E (clicking through the real window). `e2e-gui.sh` only proves the window process starts and hands off.
- Real agent launches (paid calls) were never run; `process.Launch` is only tested for error paths and command construction. Terminal emulators were not launched in tests.
- Coverage is low in `service` (40%), `storage` (43%), `process` (44%): Wails dialog/clipboard calls and the actual process start are not unit-testable without a window.
- Windows and macOS.
