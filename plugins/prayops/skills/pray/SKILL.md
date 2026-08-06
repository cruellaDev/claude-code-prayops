---
name: pray
description: Send one prayer and draw the censer in the conversation, with the prayer emoji scattered around it.
disable-model-invocation: true
---

Send one prayer and show it.

There is nothing to install: the runtime ships inside the plugin and a session
start copies it into place. Call it through `"${CLAUDE_PLUGIN_ROOT}/bin/prayops"`
rather than a path built from `$CLAUDE_PLUGIN_DATA` - that variable is only set
per plugin inside a hook, and in the Bash tool it carries whatever the parent
process had. The launcher resolves the real one.

1. Send the prayer:
   - with no argument:
     `"${CLAUDE_PLUGIN_ROOT}/bin/prayops" pray --host claude --cwd "$PWD" --preset deploy`
   - with text:
     `"${CLAUDE_PLUGIN_ROOT}/bin/prayops" pray --host claude --cwd "$PWD" --text "<user text>"`
   - with an explicitly provided image path:
     `"${CLAUDE_PLUGIN_ROOT}/bin/prayops" pray --host claude --cwd "$PWD" --image "<path>"`
2. Draw the censer where the user is looking. The status line is narrow and
   easy to miss, so a prayer is shown in the conversation too:
   `printf '{"session_id":"%s"}' "$CLAUDE_CODE_SESSION_ID" | COLUMNS=100 NO_COLOR=1 "${CLAUDE_PLUGIN_ROOT}/bin/prayops" statusline claude`
3. Print that output verbatim inside a fenced code block, before anything else
   you say. Do not re-align it, trim it, or describe it - the spacing is the
   picture. If the command printed nothing, say the prayer was sent and move on.
4. Never search for an image or inspect image contents for reasoning.
5. Do not include prompts, code, commands, or tool output beyond the censer.
6. Keep any words after the censer to one line.
