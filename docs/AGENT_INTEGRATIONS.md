# Agent integrations

Each adapter is an `agents.Profile` (see `internal/agents/builtins.go`). **Verified** = argument names checked against the installed CLI's own `--help` on the stated version on the development machine. A verified adapter has still **not** been exercised with a real paid session.

| ID | CLI | Verified | New session | Resume | Context handoff | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| `claude` | `claude` | 2.1.296 | `claude --session-id <uuid> "<prompt>"` | `claude --resume <uuid>` | prompt argument | Reads `CLAUDE.md` |
| `kilo` | `kilo` / `kilocode` | 7.4.21 | `kilo --prompt "<prompt>"` | `kilo --session <id>` / `--continue` | prompt argument | Own session IDs |
| `agy` | `agy` | 1.2.14 | `agy --prompt-interactive "<prompt>"` | `agy --conversation <id>` / `--continue` | prompt argument | Own conversation IDs |
| `freebuff` | `freebuff` | 0.0.163 | `freebuff` | `freebuff --continue [id]` | **file** (no prompt flag) | Context file path shown to user |
| `codex` | `codex` | **no** | `codex "<prompt>"` | `codex resume <id>` / `resume --last` | prompt argument | From upstream docs; not installed here |
| `gemini` | `gemini` | **no** | `gemini -i "<prompt>"` | none claimed | prompt argument | From upstream docs; not installed here |

`<prompt>` is a fixed short instruction: read the context package at `<file>`, verify claims, wait for instructions. The package itself is a file in the app data directory, never argv.

Statuses on the **AI Agents** screen: `Detected`, `Not installed`, `Needs configuration` (bad override path), `Unsupported/unknown`. The screen also shows the detected path/version, launch mode, an access warning (the agent can edit files and run commands and sends context to its provider), and **Test launch…** (a dry-run plan; a real launch needs confirmation).

## Adding an adapter without touching core code

Built-in: add a `Profile` to `Builtins()` and keep `TestBuiltinsValid` green. User/community: **AI Agents → Add a custom CLI profile** with JSON like:

```json
{ "id": "my-agent", "displayName": "My Agent", "executables": ["my-agent"],
  "newArgs": [], "promptArgs": ["--prompt", "{prompt}"], "resumeArgs": ["--resume", "{session}"],
  "handoff": "prompt-arg",
  "capabilities": { "newSession": true, "resumeById": true, "initialPrompt": true } }
```

Placeholders: `{prompt}`, `{session}`, `{cwd}`, `{contextFile}`; each template element is exactly one argument. Validation rejects bad IDs, unknown fields/placeholders, invalid env names (names only; never values) and built-in ID clashes. Imported profiles are never "verified" and nothing in them is executed except the declared executable at launch time, after your confirmation.

## Provider-neutral state

Context, sessions and handoff notes are keyed by workspace, not agent, so switching agents keeps history.
