---
name: pray
description: Send one explicit text, preset, or local-image prayer effect to the PrayOps watcher.
disable-model-invocation: true
---

Send one prayer effect.

1. Ensure the runtime exists by running:
   `"${CLAUDE_PLUGIN_ROOT}/scripts/ensure-runtime.sh"`
   Exit 0 means the runtime is ready. Exit 10 means it printed an installation plan and installed nothing.
2. On exit 10, show the printed plan to the user and ask whether to install. Only after they agree, run:
   `"${CLAUDE_PLUGIN_ROOT}/scripts/ensure-runtime.sh" --yes`
   If the user declines, do not send the prayer - say it was not sent and that `/prayops:setup` can install the runtime later.
3. With no argument:
   `"${CLAUDE_PLUGIN_DATA}/bin/prayops" pray --host claude --cwd "${CLAUDE_PROJECT_DIR}" --preset deploy`
4. With text:
   `"${CLAUDE_PLUGIN_DATA}/bin/prayops" pray --host claude --cwd "${CLAUDE_PROJECT_DIR}" --text "<user text>"`
5. With an explicitly provided image path:
   `"${CLAUDE_PLUGIN_DATA}/bin/prayops" pray --host claude --cwd "${CLAUDE_PROJECT_DIR}" --image "<path>"`
6. Never search for an image or inspect image contents for reasoning.
7. Do not include prompts, code, commands, or tool output.
8. Confirm briefly after the prayer request is accepted.
