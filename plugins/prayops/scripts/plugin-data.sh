# Resolves which data directory belongs to PrayOps. Sourced, not executed.
#
# CLAUDE_PLUGIN_DATA is set per plugin only inside a hook. Everywhere else -
# the Bash tool, a skill's command, a plain shell - it carries whatever the
# parent process happened to have, which in practice is another plugin's
# directory. Every place that trusted it has, at some point, written PrayOps
# state into a stranger's directory: silently, with exit 0, and in the worst
# case leaving a runtime.json behind that makes the wrong directory look like
# a legitimate install forever after.
#
# So the rule lives here, once, instead of being reinvented per script.

# resolve_plugin_data prints the directory that belongs to this plugin, or
# nothing at all. Nothing means "refuse" - a caller must never fall back to
# the raw environment value, because that is the poisoned one.
resolve_plugin_data() {
  _candidate="${CLAUDE_PLUGIN_DATA:-}"
  [ -n "$_candidate" ] || return 0

  # Claude Code names these directories <marketplace>-<plugin>, so ours says
  # so. A custom directory a user passed by hand also lands here, which is
  # correct: they asked for it by name.
  case "$(basename "$_candidate")" in
    *prayops*)
      printf '%s\n' "$_candidate"
      return 0
      ;;
  esac

  # The name does not say prayops. Only inside Claude Code's own
  # <root>/plugins/data/<name> layout does that mean a leak: that is where
  # sibling plugins keep their directories and where an inherited value comes
  # from. Anywhere else the path was chosen deliberately - a test harness, a
  # manual run - and obeying it beats guessing.
  _parent="$(dirname "$_candidate")"
  if [ "$(basename "$_parent")" != "data" ] ||
    [ "$(basename "$(dirname "$_parent")")" != "plugins" ]; then
    printf '%s\n' "$_candidate"
    return 0
  fi

  _found=""
  _count=0
  for _sibling in "$_parent"/*prayops*; do
    [ -d "$_sibling" ] || continue
    _found="$_sibling"
    _count=$((_count + 1))
  done

  # More than one candidate means guessing, and guessing writes somewhere
  # wrong. Refusing is recoverable; a misplaced install is not.
  [ "$_count" -eq 1 ] || return 0
  printf '%s\n' "$_found"
}
