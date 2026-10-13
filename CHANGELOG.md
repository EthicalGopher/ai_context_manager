# Changelog

## 0.2.0 — unreleased

Product pivot to a ChatGPT-style app.

- Chat with your AI CLI: replies posted as messages (Claude Code, Kilo, AGY), queue + Stop, edits opt-in; attachments (images, audio, video, PDFs) by picker, drop or paste.
- Embedded terminal (PTY + xterm.js) per chat.
- Pages: rich project documents with embedded media, versions, numbered list, Files/Media/Tools/Help panel; Save to page from any message.
- Skills & memory: always-on skills, living notes with history, one-click managed block in AGENTS.md/CLAUDE.md/GEMINI.md.
- AI tools via MCP (`aicw mcp`) and `aicw tool`: conversations, messages, media, pages, notes, skills.
- Voice dictation with local speech-to-text and one-click offline setup.
- Fixes: modals stole focus on every keystroke; a user-excluded README leaked through the "Purpose" line; faster-whisper/PyAV incompatibility worked around by decoding our own WAV.
- Page composer: add text with or without images, video, audio and PDFs to any page; voice messages kept as audio attachments with transcripts.
- Chat send target (Terminal / Chat reply), terminal opens directly and auto-starts the agent (setting).
- Schema v4.

## 0.1.0

UI redesign (icon rail + workspace browser + property header card, Urbanist/JetBrains Mono fonts bundled locally, tabbed cards, Board/Timeline session views) and a **Media** library.

First working version (Linux).

- `aicw` CLI: open/resume workspace, `--new`, `doctor`, `--help`, `--version`; paths with spaces, `~`, relative paths; file-path prompt; single-instance hand-off over a Unix socket.
- Resume-first workspace and session persistence in SQLite (WAL, busy timeout, pre-migration backup).
- Read-only scanner (instruction/skill/doc discovery, `.gitignore`, ignore rules, size/count/depth limits, symlink containment, secret and prompt-injection heuristics).
- Deterministic, budgeted Markdown context package; export/copy; user exclusions.
- Guarded file writes: preview diff, confirmation, stale-preview detection, backup, audit log, restore, archive/undo.
- Agent registry with six built-in adapters (four verified, two unverified) and validated custom profiles; shell-free launch into a terminal emulator.
- Skills: discovery, templates (maintenance, handoff, context steward), install preview, local import preview.
- Handoff notes that record only supplied facts.
- Frontend: Overview, Workspaces, Fresh Start, Context, Skills, AI Agents, Session History, Settings; dark/light themes.
- Tarball packaging, install/uninstall scripts, CI and release workflows (untested on GitHub).
- Media: import images, video, audio, PDFs and other files (dialog or drag-and-drop) as private copies (originals untouched, 2 GB per file, secret-looking files refused); gallery with filters, in-app image/video/audio preview with seeking, notes, optional inclusion of paths and notes in the context package, confirmed delete.
- `aicw --page <name>` opens a specific page; Favourites (pinned workspaces) and per-workspace session counts in the sidebar.
- Database schema v2 (media table) with automatic pre-migration backup.
