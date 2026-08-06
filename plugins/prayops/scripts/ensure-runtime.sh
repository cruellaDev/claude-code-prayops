#!/usr/bin/env sh
# Puts the runtime where the status line can find it.
#
# The binary already shipped inside the plugin, so this is a copy, not an
# installation: nothing is downloaded, nothing new is trusted. The rule that
# hooks must never install a runtime was about fetching code from the
# internet; copying a file out of the plugin the user just installed is not
# that, which is why this can run unattended from SessionStart.
#
# The copy exists because the status line setting needs one stable absolute
# path, and the plugin's own directory carries its version - it moves on every
# update, the data directory does not.
#
# Exit codes:
#    0  the runtime is in place, or was already
#    1  nowhere to put it
#   11  no binary for this platform
set -eu

PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"

# Never the raw environment value. A skill telling the model to run this from
# the Bash tool - which both the doctor and setup skills do as a repair step -
# hands it another plugin's directory, and installing there writes a
# runtime.json that makes the wrong directory look legitimate from then on.
. "$PLUGIN_ROOT/scripts/plugin-data.sh"
PLUGIN_DATA="$(resolve_plugin_data)"

if [ -z "$PLUGIN_DATA" ]; then
  cat >&2 <<EOF
PrayOps: cannot tell which data directory is ours.
CLAUDE_PLUGIN_DATA=${CLAUDE_PLUGIN_DATA:-<unset>}
Nothing was installed. Inside Claude Code this is set for you; from a shell,
set it to the plugin's own data directory.
EOF
  exit 1
fi

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) echo "PrayOps: $(uname -s) is not supported." >&2; exit 11 ;;
esac

case "$(uname -m)" in
  arm64 | aarch64) arch=arm64 ;;
  x86_64 | amd64) arch=amd64 ;;
  *) echo "PrayOps: $(uname -m) is not supported." >&2; exit 11 ;;
esac

source_bin="$PLUGIN_ROOT/runtime/${os}_${arch}/prayops"
if [ ! -f "$source_bin" ]; then
  echo "PrayOps: the plugin ships no binary for ${os}/${arch}." >&2
  exit 11
fi

version="$(sed -n 's/.*"runtimeVersion"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
  "$PLUGIN_ROOT/runtime-manifest.json")"

# The two happen to match today only because a test forces them to. Read the
# plugin's own version rather than repeating the runtime's under its name.
plugin_version() {
  sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
    "$PLUGIN_ROOT/.claude-plugin/plugin.json" | head -1
}
target="$PLUGIN_DATA/bin/prayops"
record="$PLUGIN_DATA/runtime.json"

# Reading a version string beats running the binary: SessionStart hooks are on
# a timeout, and this runs on every session.
installed=""
if [ -f "$record" ]; then
  installed="$(sed -n 's/.*"runtimeVersion"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$record")"
fi
if [ -x "$target" ] && [ "$installed" = "$version" ]; then
  exit 0
fi

mkdir -p "$PLUGIN_DATA/bin" "$PLUGIN_DATA/tmp"

# Copy then rename, so a session that starts mid-update never sees a partial
# binary - and so the running status line keeps executing the old inode until
# the new one is complete.
staged="$PLUGIN_DATA/tmp/prayops.$$"
# This runs from a hook on a timeout, and a 4.5MB copy on a slow home
# directory can be killed part way. Without this each one strands a file.
trap 'rm -f "$staged"' EXIT INT TERM
rm -f "$PLUGIN_DATA/tmp"/prayops.*
cp "$source_bin" "$staged"
chmod 0755 "$staged"

if ! "$staged" doctor --bootstrap-smoke >/dev/null 2>&1; then
  rm -f "$staged"
  echo "PrayOps: the shipped binary does not run on this machine." >&2
  exit 11
fi

mv -f "$staged" "$target"

# Written the same way, because a crash between the two leaves a binary the
# runtime cannot recognise as its own - and then it falls back to the
# environment, which is the value this whole script exists to distrust.
cat > "$record.new" <<EOF
{
  "runtimeVersion": "$version",
  "pluginVersion": "$(plugin_version)",
  "platform": "${os}/${arch}",
  "source": "plugin"
}
EOF
mv -f "$record.new" "$record"
