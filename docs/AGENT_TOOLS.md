# AI tools, chat, pages, skills and voice

## Adding data and sending messages

- **Page composer:** every page has a composer under its content. Type text, attach/paste/drop images, video, audio or PDFs, or record a voice message, and press Enter: the text and embeds are appended to the page (a version is kept each time). No Markdown needed.
- **Send target in chats:** a switch next to the attach button chooses **Terminal** (type the message into the agent running in the terminal; if none is running AICW starts the chat's agent first, waits ~2.5 s for its interface, then types it) or **Chat reply** (run the agent in the background and post its reply as a message). If the agent cannot start, the reason is shown and the message is sent as a chat reply instead.
- **Terminal opens directly:** each chat shows the terminal and, with *Settings → Open the agent terminal automatically* (default on), starts the chat's agent when the chat opens (resuming its session if it has one). Starting a CLI does not use your account until you send it something. Terminals keep running when you switch chats until you press Stop.
- **Two voice buttons:** the speaker icon records a **voice message** that is kept as an audio attachment (plus a transcript in the box when a speech engine is set up); the mic icon **dictates** (transcript only, audio discarded). The agent receives the audio file's path and, if present, the transcript text — CLI agents generally cannot listen to audio themselves, so the transcript is what they act on.

## How a message becomes a reply

1. You send a message (text, images, audio, video, PDFs). It is stored with its attachments.
2. If a terminal is running in that chat, the message is typed into it as a bracketed paste (attachments as `[attached image: /path]` lines).
3. Otherwise AICW runs the chat's agent **non-interactively** in the project folder and posts its stdout as the reply:

| Agent | First turn | Later turns | File edits (only if allowed) | Notes |
| --- | --- | --- | --- | --- |
| Claude Code | `claude -p` (prompt on stdin) `--session-id <uuid>` `--mcp-config … --append-system-prompt …` | `claude -p --resume <uuid>` | `--permission-mode acceptEdits` | exact session continuity |
| Kilo | `kilo run "<prompt>"` | `kilo run --continue "<prompt>"` | `--auto` | continues its *most recent* session in the folder; may mix if you also use Kilo there |
| AGY | `agy --print "<prompt>"` | `agy --continue --print "<prompt>"` | `--mode accept-edits` | same caveat |
| Codex, Gemini, Freebuff | no chat mode | | | use the terminal |

Flags above were checked against each CLI's own `--help`. Replies run with a 20-minute timeout, one at a time per chat (extra messages queue), and **Stop** kills the whole process group. The reply is plain text (Markdown rendered). Errors, a missing CLI and "no output" are shown as system notes, never as fake answers. By default chat replies cannot edit files; the header switch turns edits on (the terminal always uses the agent's own permission prompts).

## Tools the AI can call

Exposed identically as an MCP server (`aicw mcp --workspace <path>`, newline-delimited JSON-RPC over stdio) and as `aicw tool <name> '<json>'` (needs `AICW_WORKSPACE`, which AICW's terminal sets).

| Tool | Purpose |
| --- | --- |
| `read_context` | notes + always-on skills (call first in every session) |
| `list_notes` `read_note` `write_note` `append_note` | living notes; every replace keeps the old version |
| `list_skills` `read_skill` `add_skill` | skills; agent-written skills are labelled “written by agent” |
| `list_pages` `read_page` `write_page` `append_page` | rich pages; `![caption](/abs/path.png)` is auto-stored as media |
| `list_conversations` `create_conversation` `read_conversation` `add_message` | start chats and post updates (attachments by absolute path) |
| `add_media` `list_media` | media library |

Limits and guarantees (tested): 64 KB per message, 256 KB per note/page/skill, 10 attachments per message, names validated, unknown JSON fields rejected, a workspace can only touch its own data, **agents can post only as the assistant (never forge your messages)**, secret-looking files are refused as media, and none of these tools can start processes, type into your terminal, or write project files. The CLI/MCP process opens AICW's SQLite database directly (WAL mode), so it works even when the window is closed; the app picks up changes within a few seconds.

Claude Code receives the MCP config and the always-on rules automatically when launched by AICW (chat turns and the terminal). For other CLIs, install your always-on skills into `AGENTS.md` (Skills & memory → Install…); the block teaches them the `aicw tool` commands.

## Always-on skills

Skills live in AICW's database (per project) and are rendered into one block:

```
<!-- AICW:BEGIN … -->  rules + always-on skill bodies + tool list  <!-- AICW:END -->
```

Installing it previews a diff, requires confirmation, backs up the file, and replaces only the block (everything else is untouched, repeated installs are idempotent). The status shows *up to date / outdated* when skills change. This is how the AI is *forced* to read and update context: rules 1–3 tell it to call `read_context` first and to update notes/skills after progress. An agent that ignores its instruction file can still ignore them; AICW cannot make a CLI obey.

## Pages

Markdown documents with embedded media: `![caption](media:ID)` renders as image, video or audio (the caption becomes a badge). Remote images and links are never loaded or followed. Pages keep versions on every replace (History → restore). “Save to page” on any chat message appends its text and attachments. The right panel browses the project files read-only (secrets flagged and not previewed) and the media library.

## Voice

The mic records 16 kHz mono 16-bit WAV with `pw-record`, `parecord`, `arecord` or `ffmpeg` (first found), stops with SIGINT so the header is valid, transcribes locally, **deletes the audio**, and puts the text into the composer for you to edit; it is never auto-sent. Engines, in order: your custom command (Settings; `{audio}` placeholder, argv-split, no shell), AICW's private faster-whisper environment (Settings → Set up offline voice: `python3 -m venv` + `pip install faster-whisper`, then the model downloads on first use), a system faster-whisper, the `whisper` CLI. Transcripts are lightly cleaned (noise tags removed, whitespace, first letter capitalised); accuracy depends on the model, microphone and language.
