# Security model

## Trust boundaries

| Source | Trust |
| --- | --- |
| Repository files, skills, instruction files, AI-generated summaries | **Untrusted data.** Never executed; commands inside them are never run by AICW. |
| Frontend (webview) | Not a boundary: the backend re-validates workspace IDs, paths and options. |
| Imported agent definitions | Untrusted configuration: validated JSON only, unknown fields rejected, never marked verified. |
| Agent CLIs | Run with the user's privileges and permissions; they can edit files and run commands. |

## Safeguards (implemented and tested)

- Scanning is read-only; opening a workspace creates no `.aicw/` directory.
- Writes/moves/restores require `confirmed=true`, are previewed as a diff/plan, reject stale previews, back up the previous content, and append to an audit log (`<data>/history/<id>/audit.jsonl`). Backups/audit are outside the project.
- Paths: `~` expansion, canonicalisation, containment (`SafeJoin`) including symlink escapes; destination-exists checks on moves.
- Secrets: file-name heuristics (`.env*`, keys, tokens, credentials, tfstate, …) and content regexes (private keys, AWS/GitHub/Slack/Google/OpenAI-style tokens, `password=` patterns). Matches are excluded from the context package and from the UI file viewer. **This is heuristic, not perfect.**
- Prompt injection: phrases such as "ignore previous instructions" or `curl … | sh` in repository files raise a warning and are labelled untrusted in the package. **Absence of a warning proves nothing.**
- Processes: argv arrays only, no shell; the working directory is the validated workspace; environment variable *names* only for custom profiles, no secret values stored. The copyable "display" command is for humans and is never executed.
- IPC socket is `0600` in the data directory; a second CLI cannot make the app run anything, only open a validated directory.
- Limits: file count, size, depth (scanner); import limits (50 files / 2 MB); symlinks in skills are not followed outside their base.

- Media: imported files are copied into an owner-only store; directories and secret-looking files are refused; the `/media/<id>` handler serves only database-known IDs from inside the store with `nosniff` and a sandboxing CSP; deleting removes only AICW's copy.

- Agent tools: workspace-scoped, size-limited, schema-strict; assistant-only authorship; no process/terminal/file-write capability. MCP/CLI processes run with the user's privileges and read the same database, so anything that can run `aicw` as you can use them.
- Chat turns run the agent **without** file-edit permission unless you enable it; `--permission-mode acceptEdits` / `--auto` / `--mode accept-edits` are added only then. The agent can still read files it has access to and send them to its provider.
- Terminal: starts only on a click, argv array (never a shell string), in the validated project folder; `aicw` is put on its PATH and `AICW_WORKSPACE` is set. A plain shell runs with your normal privileges.
- Voice: audio is recorded to a private temp folder, transcribed locally, and deleted; the offline setup downloads packages/models from PyPI/Hugging Face only when you confirm.
- Pages render Markdown with inert links; remote images are not loaded; embedded media is served only from the private store by database ID.

## External data flow

AICW itself makes **no network calls**. When you launch an agent, the context package path and whatever the agent reads are sent to that agent's provider by the agent. The launch dialog states this before every launch.

## Known limitations

- `.gitignore` support is simplified. Secret and injection detection are heuristics.
- A malicious repository can still contain instructions an agent may follow; review skills and instruction files.
- PID-based "already running" detection is unreliable for terminal emulators that daemonize; the guard can miss a live session (it errs toward letting you launch with confirmation).
- The context file written for launch is readable by your user account (`0600`) until you delete it.
- No sandboxing of agents; use each CLI's own permission controls.
