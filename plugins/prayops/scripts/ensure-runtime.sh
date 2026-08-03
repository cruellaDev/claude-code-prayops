#!/usr/bin/env sh
set -eu

runtime="${CLAUDE_PLUGIN_DATA:-}/bin/prayops"

if [ -x "$runtime" ]; then
  exit 0
fi

# Arguments are passed through so a caller that already has the user's consent
# can run this with --yes. Without it setup.sh only prints its plan.
exec "${CLAUDE_PLUGIN_ROOT}/scripts/setup.sh" "$@"
