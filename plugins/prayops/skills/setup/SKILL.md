---
name: setup
description: Install, update, or repair the local PrayOps runtime and optionally configure the Claude Code status line and /pray alias.
disable-model-invocation: true
---

Set up PrayOps explicitly.

1. Run:
   `"${CLAUDE_PLUGIN_ROOT}/scripts/setup.sh"`
   This installs nothing. It prints the installation plan and exits 10 when consent is still needed, or exits 0 when the runtime is already up to date.
2. Show the printed plan to the user - runtime version, OS/architecture, GitHub Release source, asset, install location, and SHA-256 verification - and ask whether to proceed.
3. Only after the user agrees, run:
   `"${CLAUDE_PLUGIN_ROOT}/scripts/setup.sh" --yes`
   Never pass `--yes` before the user has seen the plan and agreed.
4. If it fails, report the exit code meaning rather than retrying: 11 unsupported platform, 12 missing curl/wget or sha256 tool, 13 checksum mismatch, 14 rejected archive, 15 smoke test failure, 17 another setup running. In every one of these cases nothing was installed and any existing runtime is untouched.
5. After installation, run:
   `"${CLAUDE_PLUGIN_DATA}/bin/prayops" doctor`
6. Ask separately whether to configure the compact Claude Code status line:
   - `"${CLAUDE_PLUGIN_DATA}/bin/prayops" statusline status` reports whether one is already configured.
   - `"${CLAUDE_PLUGIN_DATA}/bin/prayops" statusline install` changes nothing and exits 10 after printing what it would write, including any existing status line it would replace.
   - Show that to the user. Only after they agree, run the same command with `--yes`.
   - `"${CLAUDE_PLUGIN_DATA}/bin/prayops" statusline uninstall --yes` restores what was there before.
7. Ask separately whether to install the optional `/pray` alias.
8. Never overwrite an existing status line or alias without backup and confirmation.
