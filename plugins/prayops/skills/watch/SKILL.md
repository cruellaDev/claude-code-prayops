---
name: watch
description: Show how to run the full PrayOps pixel altar in a separate terminal or tmux pane.
disable-model-invocation: true
---

Never start the PrayOps altar from the Bash tool. It runs until the user quits, so it would hold the tool open for the rest of the session. This skill only tells the user how to start it themselves.

1. Check whether the runtime is installed at `${CLAUDE_PLUGIN_DATA}/bin/prayops`. If it is not, say so and point at `/prayops:setup` instead of showing the commands below.

2. Report whether a watcher is already running:
   `"${CLAUDE_PLUGIN_DATA}/bin/prayops" doctor` includes a `Watcher` line.
   A watcher that is not running is the normal state, not a problem.

3. Show the command with the absolute path, because `prayops` is not on the user's PATH:

   ```bash
   "<plugin-data>/bin/prayops" watch
   ```

   `CLAUDE_PLUGIN_DATA` is set inside Claude Code but not in the user's own shell, so substitute the real path into the command you show them.

4. If tmux is available, offer the split as an option, never as the default:

   ```bash
   tmux split-window -h '<plugin-data>/bin/prayops watch'
   ```

   Do not install tmux and do not run it for them.

5. Mention the two flags that matter:
   - `--motion off` stills the smoke and steps the dissolve, for reduced motion.
   - `NO_COLOR` does the same and drops colour.

6. Explain the split: the Claude Code status line is a one-line snapshot, and the incense smoke and prayer dissolve only appear in the separate watcher.

7. `q`, `esc`, or `ctrl+c` quits it and restores the terminal.
