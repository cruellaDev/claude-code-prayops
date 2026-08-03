---
name: setup
description: Install, update, or repair the local PrayOps runtime and optionally configure the Claude Code status line and /pray alias.
disable-model-invocation: true
---

Set up PrayOps explicitly.

1. Run:
   `"${CLAUDE_PLUGIN_ROOT}/scripts/setup.sh"`
2. Before executing network installation, show the user the runtime version, OS/architecture, GitHub Release source, install location, and SHA-256 verification.
3. Do not install silently.
4. After installation, run:
   `"${CLAUDE_PLUGIN_DATA}/bin/prayops" doctor`
5. Ask separately whether to configure:
   - compact Claude Code status line
   - optional `/pray` alias
6. Never overwrite an existing status line or alias without backup and confirmation.
