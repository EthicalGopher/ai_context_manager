# CLI

```text
aicw                  open the app (recent workspaces, resumable sessions)
aicw .                open or resume the workspace in the current directory
aicw <path>           absolute, relative, or ~/… ; spaces work when quoted
aicw --page <name> .  open a specific page (overview, workspaces, fresh-start, context,
                      media, skills, agents, sessions, settings)
aicw --new <path>     request a new Fresh Start; existing sessions are kept
aicw doctor           diagnostics (exit 1 if any check fails)
aicw --help | --version
```

- `aicw .` **resumes by default** and never starts an AI session. See [SESSION_PERSISTENCE](SESSION_PERSISTENCE.md).
- Invalid paths print an error, exit 1 and create nothing. Unknown flags exit 2.
- A file path asks "open its parent directory?" on an interactive terminal; non-interactively it is rejected.
- If AICW is already running the request is delivered to it, the window is raised, and the CLI exits 0. Otherwise the app starts.
- Use `--` to open a directory whose name starts with `-`.

## Install and PATH

```bash
./install.sh                      # ~/.local/bin/aicw, ~/.local/share/applications/aicw.desktop
PREFIX=/opt/aicw ./install.sh     # custom prefix (needs write access)
# or from a source checkout:
scripts/build.sh && ln -s "$PWD/build/bin/aicw" ~/.local/bin/aicw
```

Ensure `~/.local/bin` is on `PATH` (`export PATH="$HOME/.local/bin:$PATH"` in `~/.bashrc`/`~/.zshrc`; fish: `fish_add_path ~/.local/bin`). Development (`wails dev`) is separate from the installed CLI.

## Upgrade / uninstall

Re-run `./install.sh` from the new tarball (binary is replaced atomically; data is untouched). `./uninstall.sh` removes program files only; `./uninstall.sh --purge-data` asks you to type `delete` before removing `~/.local/share/aicw`. Project files, `.aicw/` folders and project skills are never removed.

## Troubleshooting

| Symptom | Fix |
| --- | --- |
| `aicw: command not found` | Add the install `bin` directory to `PATH`; run `~/.local/bin/aicw doctor` |
| `doctor`: "aicw on PATH … different binary" | An older copy earlier on `PATH`; `which -a aicw` |
| Blank window / WebKit errors | Install `webkit2gtk-4.1`; try `WEBKIT_DISABLE_COMPOSITING_MODE=1 aicw .` |
| Launch says "not installed" | Install the CLI or set its path in **AI Agents** |
| No terminal found | Install a supported emulator or copy the command shown in the launch dialog |
| Data location | `~/.local/share/aicw` (`AICW_DATA_DIR` overrides) |
