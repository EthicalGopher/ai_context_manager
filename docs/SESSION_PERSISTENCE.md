# Session persistence and resume-first behaviour

**Identity.** A workspace is the canonical path (symlinks resolved). Its record stores last agent, task notes, exclusions and mode. Sessions are separate rows: agent, external (CLI) session ID, status, title, handoff summary, context-package version, exit status, timestamps, launcher PID. **Transcripts are not stored.**

**`aicw .` flow** (`workspace.Manager.Open`, tested):

| Saved sessions (not completed) | Result shown | Starts anything? |
| --- | --- | --- |
| 0 | "No saved session" → Fresh Start | No |
| 1 | Resume panel with details | No (needs click + confirmation) |
| >1 | Picker (agent, last active, status, handoff line) | No |
| any, with `--new` | Fresh Start wizard; old sessions untouched | No |

Opening repeatedly never creates sessions (tested, including 30 concurrent opens/writes).

**States:** `active`, `paused`, `completed`, `interrupted`, `needs_attention`, `external_session_unavailable`. `completed` sessions are not offered for resume. Change status on **Session History**.

**Resume mechanics per CLI** (details in [AGENT_INTEGRATIONS](AGENT_INTEGRATIONS.md)):

- Claude Code: AICW pre-assigns `--session-id <uuid>` on new sessions, so `--resume <uuid>` resumes exactly that conversation.
- Kilo/AGY/Freebuff: they choose their own IDs, which AICW cannot read, so a saved session has no external ID. Resume falls back to the CLI's "continue most recent" and the dialog says it may not be the same conversation.
- Gemini (unverified) has no claimed resume: AICW refuses and offers a new session with the saved handoff.
- AICW never claims a conversation was restored when only its own context was.

**Duplicate prevention.** (1) Second `aicw` invocations are delivered to the running window over IPC (plus Wails' lock). (2) A session marked `active` whose recorded launcher PID is alive is not launched again without an explicit "Launch anyway". Limitation: terminal emulators that daemonize exit immediately, so a live agent can look dead; AICW cannot see inside the terminal.

**Concurrency.** Two projects never share records (tested). SQLite WAL + `busy_timeout`, one connection per process.
