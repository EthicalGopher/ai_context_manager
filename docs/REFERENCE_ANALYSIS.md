# Reference analysis: `witnesstodark/mr-mak-workspace`

Cloned at `--depth 1` (version 0.5.1 in `package.json`, MIT license). **Scope of review:** I read the README, `package.json`, the directory layout, `docs/skills.md`, and grepped `desktop/service/agents.mjs` and related files for launch/resume behaviour. I did **not** read every source file or run its tests, so anything below not marked *Observed* is an assumption.

## Observed facts

- Stack: Tauri (Rust, `src-tauri/`) + React 19 + Vite + xterm.js in `src/`; a Node service in `desktop/service/` (`sessions.mjs`, `agents.mjs`, `server.mjs`, …) with tests under `desktop/service/test/`.
- Two desktop windows: **Chats** (each chat is a real CLI in an embedded terminal, saved history) and **Workspace** (files, Markdown viewer/editor, skills, MCP, settings).
- Agents: Codex, Claude Code, OpenCode, Kimi. `terminalCommand()` builds per-agent argv and resumes with `codex resume <id>` / `claude --resume <id>` / `--session <id>` (OpenCode).
- The app owns a *workspace folder template* (`inbox`, `projects`, `workspace`, `knowledge`, `processes`) rather than opening arbitrary directories.
- 20 skills live in `.agents/skills` (source for Codex) and identical copies in `.claude/skills`, kept in sync by `scripts/sync-skills.mjs` and checked by a test.
- Mobile access via Tailscale and voice dictation; Windows-first setup (`Setup.ps1`, `Start Mr. Mak.cmd`) with a Linux AppImage path.

## Useful ideas adopted (re-implemented, not copied)

- Real CLI processes instead of simulated agents; per-agent launch builders instead of one shared flag set.
- Saved native session IDs for resume; honest handling when an agent cannot resume.
- Markdown as a first-class, editable artifact; skills mirrored per agent format.

## Shortcomings relative to this product's goals (assumptions marked)

- Opinionated folder template instead of "open any directory with `cmd .`" (observed layout).
- Embedded terminal + Node service + Rust shell is a heavy runtime for a local manager (judgement).
- *Assumption, not verified:* no pre-launch secret filtering, diff-before-write or audit log was found in the parts I read.
- Only four agents; adding one means editing source (observed `agents.mjs` switch-style code).

## License handling

MIT permits reuse with attribution. No code was reused, so no attribution is required; the design inspiration is credited here.
