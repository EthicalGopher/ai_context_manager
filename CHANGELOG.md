# Changelog

## 0.2.0 — unreleased

Product pivot to a ChatGPT-style app.

- Chat with your AI CLI: replies posted as messages (Claude Code, Kilo, AGY), queue + Stop, edits opt-in; attachments (images, audio, video, PDFs) by picker, drop or paste.
- Elephant-Goldfish design workflow (after Google's EGM plugin) as one command, `/design`: brainstorm with you (no code), write the design doc as a page, then fresh context-free "goldfish" reviewers test it in rounds with the elephant and give a go/no-go.
- Pages: **Download HTML** saves a page as one self-contained file (media embedded, light/dark) to send to friends; images and videos open in a new viewer with smooth seeking, speed, volume, fullscreen and zoom/pan (buttons, +/-/0, Ctrl+scroll, pinch, drag).
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
- Terminal and chat share one conversation: what you type and what the agent answers in the embedded terminal is mirrored into the chat view (read from Claude Code's own session file), so you can carry on in the chat GUI afterwards. Other agents don't keep a readable transcript, so mirroring is Claude Code only. Database schema 5 adds `messages.ext_id` (backup is made first).
- Windows (experimental): ConPTY terminal, Windows Terminal/cmd launch, `scripts/build.ps1`, embedded icon.
- Fixed slash commands in the composer (`/grill-me`, `/write-pages`, `/summarize`, `/plan`, `/review`) with a "/" menu, and a **Write to pages** button that has the agent save the conversation into pages on its own. Chat replies with Claude Code now pre-approve AICW's own tools (`--allowedTools mcp__aicw`) so headless runs can actually write pages and notes.
- Per-agent model option (`--model`) in the chat header and on the AI Agents page; applies to chat replies and terminal starts. Custom agent profiles can declare `modelArgs` with a `{model}` placeholder.
- Model list: a dropdown per agent filled from `kilo models`, `agy models` and Claude's aliases. No model is pre-selected.
- Local models without Ollama: Settings → Local models downloads llama.cpp's server and a GGUF model into AICW's own folder and runs it on demand through Claude Code (`ANTHROPIC_BASE_URL`, `--strict-mcp-config`). Needs the in-app terminal or chat replies (not an external terminal).
