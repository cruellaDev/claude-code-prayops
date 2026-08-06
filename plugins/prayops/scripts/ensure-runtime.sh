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
PLUGIN_DATA="${CLAUDE_PLUGIN_DATA:-}"

if [ -z "$PLUGIN_DATA" ]; then
  echo "PrayOps: CLAUDE_PLUGIN_DATA is not set." >&2
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
cp "$source_bin" "$staged"
chmod 0755 "$staged"

if ! "$staged" doctor --bootstrap-smoke >/dev/null 2>&1; then
  rm -f "$staged"
  echo "PrayOps: the shipped binary does not run on this machine." >&2
  exit 11
fi

mv -f "$staged" "$target"
cat > "$record" <<EOF
{
  "runtimeVersion": "$version",
  "pluginVersion": "$version",
  "platform": "${os}/${arch}",
  "source": "plugin"
}
EOF
