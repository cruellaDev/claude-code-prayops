---
name: doctor
description: Diagnose PrayOps plugin, runtime, hook, status line, alias, watcher, and terminal capability.
disable-model-invocation: true
---

1. If the runtime exists, run:
   `"${CLAUDE_PLUGIN_DATA}/bin/prayops" doctor --plugin-root "${CLAUDE_PLUGIN_ROOT}" --plugin-data "${CLAUDE_PLUGIN_DATA}"`
2. If it does not exist, report:
   - plugin installed
   - runtime missing
   - run `/prayops:setup`
3. Do not download or change configuration in doctor mode.
4. Present a compact diagnostic table.
