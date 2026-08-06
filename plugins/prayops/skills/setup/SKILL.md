---
name: setup
description: Choose where the PrayOps censer appears - the Claude Code status line, this project only or everywhere - and optionally install the /pray alias.
disable-model-invocation: true
---

The runtime is not installed here. It ships inside the plugin and a session
start already copied it into place, so this skill only decides where the
censer is shown.

1. Confirm the runtime is there:
   `"${CLAUDE_PLUGIN_ROOT}/bin/prayops" doctor`
   If it reports the runtime missing, run
   `"${CLAUDE_PLUGIN_ROOT}/scripts/ensure-runtime.sh"` and then doctor again.
   That script copies a shipped binary; it downloads nothing and needs no
   confirmation. Exit 11 means this platform has no shipped binary - report
   that and stop.
2. Report whether a status line is already configured:
   `"${CLAUDE_PLUGIN_ROOT}/bin/prayops" statusline status`
3. Install the status line if it is not already there. It has to live in the
   user's `settings.json`: Claude Code reads `statusLine` from there only, and
   a copy in a project's `.claude/settings.json` or `settings.local.json` is
   ignored without a word - scoping that way hides the censer everywhere.
   `"${CLAUDE_PLUGIN_ROOT}/bin/prayops" statusline install` changes nothing and
   exits 10 after printing what it would write, including any existing status
   line it would replace. Show that, get agreement, then run the same command
   with `--yes`.
   `"${CLAUDE_PLUGIN_ROOT}/bin/prayops" statusline uninstall --yes` restores
   what was there before.
4. Ask which projects should show a censer, and set it:
   - **every project** (the default):
     `"${CLAUDE_PLUGIN_ROOT}/bin/prayops" statusline scope --everywhere`
   - **this project only**:
     `"${CLAUDE_PLUGIN_ROOT}/bin/prayops" statusline scope --only-here`
     Run it again in any other project to add that one too.
   - **not this project**:
     `"${CLAUDE_PLUGIN_ROOT}/bin/prayops" statusline scope --not-here`
   `... statusline scope` with no flag reports the current setting. Removing
   the last chosen project restores every project rather than hiding it
   everywhere.
5. If the user wants no status line at all, say `/prayops:pray` still draws
   the censer in the conversation, and skip to the next step.
6. Ask separately whether to install the optional `/pray` alias:
   - `"${CLAUDE_PLUGIN_ROOT}/bin/prayops" alias install` changes nothing and
     exits 10 after printing where it would write.
   - Only after the user agrees, run the same command with `--yes`.
   - Exit 3 means something already provides `/pray`; that file is left
     untouched and `/prayops:pray` keeps working. Do not offer to delete it.
   - `"${CLAUDE_PLUGIN_ROOT}/bin/prayops" alias uninstall --yes` removes it.
7. Finish by telling the user where the censer lives:
   - the status line, if they chose one, updates on its own from now on;
   - `/prayops:pray` draws it in the conversation and sends a prayer;
   - `/prayops:watch` runs the full animation in their own terminal, and is
     optional.
8. Never overwrite an existing status line or alias without backup and
   confirmation. Both are optional, and PrayOps works without either.
