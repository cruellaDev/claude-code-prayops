---
name: pray
description: Send one explicit text, preset, or local-image prayer effect to the PrayOps watcher.
disable-model-invocation: true
---

Send one prayer effect.

1. Ensure the runtime exists by running:
   `"${CLAUDE_PLUGIN_ROOT}/scripts/ensure-runtime.sh"`
2. If installation or update is required, show the user what will be installed and obtain confirmation.
3. With no argument:
   `"${CLAUDE_PLUGIN_DATA}/bin/prayops" pray --host claude --cwd "${CLAUDE_PROJECT_DIR}" --preset deploy`
4. With text:
   `"${CLAUDE_PLUGIN_DATA}/bin/prayops" pray --host claude --cwd "${CLAUDE_PROJECT_DIR}" --text "<user text>"`
5. With an explicitly provided image path:
   `"${CLAUDE_PLUGIN_DATA}/bin/prayops" pray --host claude --cwd "${CLAUDE_PROJECT_DIR}" --image "<path>"`
6. Never search for an image or inspect image contents for reasoning.
7. Do not include prompts, code, commands, or tool output.
8. Confirm briefly after the prayer request is accepted.
