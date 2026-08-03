---
name: watch
description: Show how to run the full PrayOps pixel altar in a separate terminal or tmux pane.
disable-model-invocation: true
---

Do not start a long-running PrayOps TUI inside Claude Code's Bash tool.

1. Check whether the runtime is installed.
2. Show:
   `prayops watch`
3. If tmux is available, optionally show:
   `tmux split-window -h 'prayops watch'`
4. Explain that the Claude Code status line is compact and the full smoke and dissolve animation appears in the separate watcher.
5. If no watcher is running, do not treat that as an error.
