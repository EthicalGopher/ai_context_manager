# Architecture

```
aicw (one binary)
 ├─ main.go                  CLI parsing, path validation, IPC hand-off, Wails bootstrap
 ├─ internal/cli             arg parsing, help/version/doctor, file-path prompt
 ├─ internal/ipc             Unix-socket hand-off to the running instance (0600)
 ├─ internal/service         the ONLY object bound to the UI (thin application layer)
 │    ├─ internal/workspace  resume-first Open(); StartSession
 │    ├─ internal/storage    SQLite: workspaces, sessions, settings, custom agents
 │    ├─ internal/scanner    read-only discovery + classification + Git info
 │    ├─ internal/contextbuilder  deterministic budgeted Markdown package
 │    ├─ internal/markdown   diff, confirmed writes, backups, audit log, move/undo
 │    ├─ internal/skills     discovery, templates, import preview
 │    ├─ internal/agents     registry, profiles, detection, argv builders
 │    ├─ internal/process    terminal emulator launch (argv only)
 │    ├─ internal/history    handoff note generation
 │    ├─ internal/tools      AI-facing actions (notes, skills, pages, conversations, media)
 │    ├─ internal/mcp        MCP stdio server over tools
 │    ├─ internal/rules      always-on instruction block (render/install/extract)
 │    ├─ internal/term       PTY sessions for the embedded terminal
 │    ├─ internal/voice      recording + local speech-to-text
 │    ├─ internal/media      private media store
 │    └─ internal/safety     path resolution/containment, secret & injection heuristics
 └─ frontend/               React + TS + Tailwind; talks only to `service` via generated Wails bindings
```

## Data flow: `aicw .`

1. `main` parses args, resolves the path (`safety.ResolveDir`: `~`, relative, symlinks, must be an existing directory; nothing is created).
2. If a running instance answers on `<data>/aicw.sock`, the request is delivered and the CLI exits 0. Wails' own `SingleInstanceLock` stays enabled as a fallback.
3. Otherwise this process becomes the app: `workspace.Open` upserts the workspace by canonical path, loads saved sessions and returns an action (`resume`, `choose`, `new`). **No session is created and no agent is started.**
4. The UI shows the resume panel; launching always goes through a dry-run plan and a confirmation.

## Key decisions

- **UI is not a security boundary.** Bound methods take a workspace ID and look the path up in SQLite; relative paths are re-validated with `safety.SafeJoin` (traversal + symlink escape).
- **Lifecycle methods live on a separate unbound type** (`service.Lifecycle`) so the frontend cannot call `Close`.
- **Adapters are data.** `agents.Profile` has argument templates (`{session}`, `{contextFile}`, …); each template element is one argv entry, so no re-parsing. Imported profiles are validated, never marked verified, and cannot replace built-ins.
- **Context files go to the app data dir by default** (`<data>/contexts/<workspace>/…`), not into the project. Backups and the audit log live in `<data>/history/<workspace>/`. The project only changes when the user confirms a write.
- **Context handoff** is a short prompt pointing at the context file (so argv stays small); CLIs without a prompt flag get file handoff and a visible note.
- **Determinism:** the package contains no randomness; only the generated header carries a timestamp, which is excluded from the content hash.

## Scanner rules

`scanner.DefaultRules()` maps patterns to categories (AGENTS.md, CLAUDE.md, GEMINI.md, CODEX.md, `.cursorrules`, `.cursor/rules/`, `.github/copilot-instructions.md`, `.github/instructions/`, `.claude/`, `.codex/`, `.gemini/`, `.agents/`, `skills/`, `docs/`, `context/`, `.github/`, `.aicw/`). Rules, ignored directories/globs and the size limit are configurable (Settings; `Options.Rules` in code). Defaults skip `node_modules`, `.git`, `vendor`, `dist`, `build`, caches; honour the root `.gitignore` (simple matcher: no `**`, nested ignore files or negation across directories); skip symlinks that leave the workspace.

## Storage

`<data>/aicw.db` (default `~/.local/share/aicw`, override with `AICW_DATA_DIR`). WAL + `busy_timeout`; one connection per process serialises writers; the migration step copies the file to `aicw.db.bak-v<N>` before changing an existing database.

## Chat, terminal and tools (added in 0.2)

- `storage` schema v4 adds conversations, messages (+ attachments), notes (+ versions), skills, pages (+ versions) and media; migrations back up the database first.
- `service.SendMessage` stores the message, then either types it into the conversation's PTY or starts/queues a non-interactive chat turn (`agents.BuildChat` → `exec` with its own process group → reply stored as a message).
- `aicw mcp` / `aicw tool` are separate entry points in `main.go`; they open the same SQLite file and call `tools.Call`, so the agent side has no dependency on the window being open.
- Events: `term:data`, `term:exit`, `chat:turn`, `voice:setup`. The frontend also polls messages/conversations/pages every few seconds so agent-written data appears without a window event.
- Routes: `/` and `/c/:id` chat, `/p/:id` pages, `/skills`, `/settings`, and the older dashboard pages under the “…” menu.
