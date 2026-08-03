#!/usr/bin/env sh
set -eu

runtime="${CLAUDE_PLUGIN_DATA}/bin/prayops"

if [ -x "$runtime" ]; then
  exit 0
fi

exec "${CLAUDE_PLUGIN_ROOT}/scripts/setup.sh"
