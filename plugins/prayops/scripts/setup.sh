#!/usr/bin/env sh
set -eu

# AI implementation target:
# - read runtime-manifest.json
# - print installation source/version/platform
# - require explicit confirmation
# - download release archive and checksums
# - verify SHA-256
# - install atomically into CLAUDE_PLUGIN_DATA/bin/prayops
# - write runtime.json
# - run doctor smoke test

echo "PrayOps setup bootstrap is not implemented yet." >&2
exit 2
