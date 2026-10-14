# Packaging

| Artifact | How | Status |
| --- | --- | --- |
| `aicw-<v>-linux-amd64.tar.gz` + `.sha256` | `scripts/package.sh <v>` | **Built and install-tested** locally |
| Source install | `scripts/build.sh` then symlink | Works |
| AppImage / deb / rpm | — | Not provided |
| Windows | `scripts/build.ps1` → `build\bin\aicw.exe` | Experimental, untested on real Windows; no code signing |
| macOS | — | Not built or tested; no signing/notarization claimed |
| GitHub release workflow | `.github/workflows/release.yml` on `v*` tags | Written, **never run** |

The tarball contains the single `aicw` binary (it is both app and CLI), icon, `.desktop` entry, `install.sh`, `uninstall.sh`, README and CHANGELOG. Verify with `sha256sum -c aicw-<v>-linux-amd64.tar.gz.sha256`.

## Install / upgrade / uninstall (verified by `scripts/e2e-cli.sh`)

- Install to `~/.local` (no root) or `PREFIX=…`; `aicw doctor` then reports `aicw on PATH`.
- Upgrade = re-run `install.sh`; it prints old → new version and swaps the binary atomically. Data in `~/.local/share/aicw` is preserved; the database is copied to `aicw.db.bak-v<N>` before any schema migration.
- Uninstall removes program files only. `--purge-data` requires typing `delete`. Workspaces, sessions, generated context, backups and project skills survive otherwise.
- Not verified: reinstall after purge on a second distribution, desktop-entry icon rendering in other desktop environments.

## Build requirements

Go, Node 20+, `libgtk-3-dev`, `libwebkit2gtk-4.1-dev` (or 4.0: drop the `webkit2_41` tag). The binary dynamically links GTK/WebKit, so end users need `gtk3` and `webkit2gtk-4.1` installed.
