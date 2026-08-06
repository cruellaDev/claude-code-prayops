---
name: doctor
description: Diagnose PrayOps plugin, runtime, hook, status line, alias, watcher, and terminal capability.
disable-model-invocation: true
---

1. Run:
   `"${CLAUDE_PLUGIN_ROOT}/bin/prayops" doctor --plugin-root "${CLAUDE_PLUGIN_ROOT}"`
   Use that launcher rather than a path built from `$CLAUDE_PLUGIN_DATA`. That
   variable is only set per plugin inside a hook; in the Bash tool it carries
   whatever the parent process had, which is often another plugin's directory.
   The launcher resolves the real one.
2. If it reports the runtime missing, run
   `"${CLAUDE_PLUGIN_ROOT}/scripts/ensure-runtime.sh"` and try again. That
   copies a binary that shipped with the plugin and downloads nothing.
3. Do not change configuration in doctor mode.
4. Present a compact diagnostic table.
