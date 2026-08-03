#!/usr/bin/env sh
#
# PrayOps runtime bootstrap.
#
# Downloads the release archive for this platform from GitHub Releases,
# verifies its SHA-256 against checksums.txt, and installs it atomically into
# ${CLAUDE_PLUGIN_DATA}/bin/prayops. An existing runtime survives every failure
# path.
#
# Without --yes this prints the installation plan and installs nothing, so a
# caller running without a terminal can show the plan to the user first.
#
# Exit codes:
#    0  installed, or already up to date
#   10  consent required - the plan was printed, nothing was installed
#   11  this OS/architecture is not supported
#   12  no download tool (curl/wget) or no checksum tool (sha256sum/shasum)
#   13  checksum mismatch
#   14  release archive rejected by the entry allowlist
#   15  the freshly downloaded binary failed its smoke test
#   17  another setup is already running
#    1  any other failure
set -eu

PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-}"
if [ -z "$PLUGIN_ROOT" ]; then
  PLUGIN_ROOT="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
fi

PLUGIN_DATA="${CLAUDE_PLUGIN_DATA:-}"
if [ -z "$PLUGIN_DATA" ]; then
  echo "PrayOps: CLAUDE_PLUGIN_DATA is not set. Run this from Claude Code via /prayops:setup." >&2
  exit 1
fi

MANIFEST="$PLUGIN_ROOT/runtime-manifest.json"
if [ ! -f "$MANIFEST" ]; then
  echo "PrayOps: runtime manifest not found at $MANIFEST" >&2
  exit 1
fi

assume_yes=0
for arg in "$@"; do
  case "$arg" in
    --yes | -y) assume_yes=1 ;;
    *)
      echo "PrayOps: unknown option $arg" >&2
      exit 1
      ;;
  esac
done

# ponytail: the manifests are files we ship ourselves, so a scalar-only reader
# beats requiring jq on the user's machine. Switch to jq if the manifest ever
# grows nested values that matter here.
json_string() {
  sed -n 's/.*"'"$2"'"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$1" | head -n 1
}

RUNTIME_VERSION="$(json_string "$MANIFEST" runtimeVersion)"
REPOSITORY="$(json_string "$MANIFEST" repository)"
ASSET_TEMPLATE="$(json_string "$MANIFEST" assetTemplate)"
CHECKSUM_ASSET="$(json_string "$MANIFEST" checksumAsset)"

if [ -z "$RUNTIME_VERSION" ] || [ -z "$REPOSITORY" ] || [ -z "$ASSET_TEMPLATE" ] || [ -z "$CHECKSUM_ASSET" ]; then
  echo "PrayOps: runtime manifest is incomplete" >&2
  exit 1
fi

PLUGIN_VERSION="$(json_string "$PLUGIN_ROOT/.claude-plugin/plugin.json" version)"

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;; # WSL reports Linux and is treated as Linux.
  *) os="$(uname -s)" ;;
esac

case "$(uname -m)" in
  arm64 | aarch64) arch=arm64 ;;
  x86_64 | amd64) arch=amd64 ;;
  *) arch="$(uname -m)" ;;
esac

# The supported list is an array of {"os":..,"arch":..} objects; collapse the
# whitespace so one grep can answer whether this pair is in it.
if ! tr -d ' \n\t' <"$MANIFEST" | grep -q "{\"os\":\"$os\",\"arch\":\"$arch\"}"; then
  cat >&2 <<EOF
PrayOps does not ship a runtime for $os/$arch.

Supported in v0.1: macOS and Linux (including WSL) on arm64 and amd64.
Native Windows is planned for v0.2.
EOF
  exit 11
fi

ASSET="$(printf '%s' "$ASSET_TEMPLATE" |
  sed -e "s/{version}/$RUNTIME_VERSION/" -e "s/{os}/$os/" -e "s/{arch}/$arch/")"

# PRAYOPS_RELEASE_BASE_URL exists so the bootstrap tests can point at a local
# server. Users never set it.
BASE_URL="${PRAYOPS_RELEASE_BASE_URL:-https://github.com/$REPOSITORY/releases/download/v$RUNTIME_VERSION}"

INSTALLED_BIN="$PLUGIN_DATA/bin/prayops"
RUNTIME_JSON="$PLUGIN_DATA/runtime.json"

installed_version=""
if [ -f "$RUNTIME_JSON" ]; then
  installed_version="$(json_string "$RUNTIME_JSON" runtimeVersion)"
fi

if [ -x "$INSTALLED_BIN" ] && [ "$installed_version" = "$RUNTIME_VERSION" ]; then
  echo "PrayOps runtime v$RUNTIME_VERSION is already installed."
  exit 0
fi

action="Installing"
if [ -n "$installed_version" ]; then
  action="Updating from v$installed_version to"
fi

cat <<EOF
PrayOps runtime

  Action    $action v$RUNTIME_VERSION
  Platform  $os/$arch
  Source    $BASE_URL
  Asset     $ASSET
  Verify    SHA-256 against $CHECKSUM_ASSET
  Install   $INSTALLED_BIN
EOF

if [ "$assume_yes" -ne 1 ]; then
  if [ -t 0 ]; then
    printf '\nProceed? [y/N] '
    read -r reply
    case "$reply" in
      y | Y | yes | YES) ;;
      *)
        echo "Cancelled. The existing runtime, if any, is untouched."
        exit 10
        ;;
    esac
  else
    cat <<EOF

Nothing has been installed. Re-run with --yes to confirm.
EOF
    exit 10
  fi
fi

if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL --proto '=https,http' -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -q -O "$2" "$1"; }
else
  cat >&2 <<EOF
PrayOps needs curl or wget to download the runtime.

Install one, or download $ASSET manually from
$BASE_URL and extract prayops into $PLUGIN_DATA/bin/.
EOF
  exit 12
fi

if command -v sha256sum >/dev/null 2>&1; then
  sha256_of() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then
  sha256_of() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
  echo "PrayOps needs sha256sum or shasum to verify the download." >&2
  exit 12
fi

# One setup at a time. mkdir is atomic on every POSIX filesystem.
LOCK="$PLUGIN_DATA/setup.lock"
mkdir -p "$PLUGIN_DATA"
if ! mkdir "$LOCK" 2>/dev/null; then
  echo "PrayOps: another setup is already running ($LOCK)." >&2
  exit 17
fi

WORK="$PLUGIN_DATA/tmp/install-$$"
cleanup() {
  rm -rf "$WORK"
  rmdir "$LOCK" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

mkdir -p "$WORK"
chmod 700 "$WORK"

echo
echo "Downloading $ASSET ..."
if ! fetch "$BASE_URL/$ASSET" "$WORK/$ASSET"; then
  echo "PrayOps: download failed. The existing runtime is unchanged." >&2
  exit 1
fi
if ! fetch "$BASE_URL/$CHECKSUM_ASSET" "$WORK/$CHECKSUM_ASSET"; then
  echo "PrayOps: could not download $CHECKSUM_ASSET. Refusing to install unverified code." >&2
  exit 1
fi

expected="$(grep " \{1,2\}\*\{0,1\}$ASSET\$" "$WORK/$CHECKSUM_ASSET" | cut -d' ' -f1 | head -n 1)"
if [ -z "$expected" ]; then
  echo "PrayOps: $CHECKSUM_ASSET has no entry for $ASSET." >&2
  exit 13
fi

actual="$(sha256_of "$WORK/$ASSET")"
if [ "$expected" != "$actual" ]; then
  cat >&2 <<EOF
PrayOps: checksum mismatch for $ASSET.

  expected  $expected
  actual    $actual

Nothing was installed and the existing runtime is unchanged.
EOF
  exit 13
fi
echo "SHA-256 verified."

# The archive must contain the binary and nothing else. Reject absolute paths,
# parent traversal, and any entry that is not a regular file.
listing="$(tar -tzf "$WORK/$ASSET")" || {
  echo "PrayOps: could not read the release archive." >&2
  exit 14
}
for entry in $listing; do
  case "$entry" in
    prayops) ;;
    *)
      echo "PrayOps: refusing archive - unexpected entry '$entry'." >&2
      exit 14
      ;;
  esac
done
if tar -tvzf "$WORK/$ASSET" | grep -qv '^-'; then
  echo "PrayOps: refusing archive - it contains a link or directory entry." >&2
  exit 14
fi

tar -xzf "$WORK/$ASSET" -C "$WORK" prayops
chmod 755 "$WORK/prayops"

# Smoke test the downloaded binary before it can replace a working one.
reported="$("$WORK/prayops" version --json 2>/dev/null |
  sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
if [ "$reported" != "$RUNTIME_VERSION" ]; then
  echo "PrayOps: downloaded binary reports version '$reported', expected '$RUNTIME_VERSION'." >&2
  exit 15
fi
if [ "$("$WORK/prayops" doctor --bootstrap-smoke 2>/dev/null)" != "ok" ]; then
  echo "PrayOps: downloaded binary failed its smoke test." >&2
  exit 15
fi
echo "Smoke test passed."

mkdir -p "$PLUGIN_DATA/bin" "$PLUGIN_DATA/state/events/tmp" \
  "$PLUGIN_DATA/state/events/inbox" "$PLUGIN_DATA/state/events/rejected" \
  "$PLUGIN_DATA/state/sessions" "$PLUGIN_DATA/state/watchers"

# rename(2) within one filesystem is atomic, so the old binary is only ever
# replaced by a fully verified one.
mv "$WORK/prayops" "$INSTALLED_BIN.new"
mv "$INSTALLED_BIN.new" "$INSTALLED_BIN"

cat >"$RUNTIME_JSON.new" <<EOF
{
  "runtimeVersion": "$RUNTIME_VERSION",
  "pluginVersion": "$PLUGIN_VERSION",
  "installedAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "asset": "$ASSET",
  "sha256": "$actual"
}
EOF
mv "$RUNTIME_JSON.new" "$RUNTIME_JSON"

echo
echo "PrayOps runtime v$RUNTIME_VERSION installed at $INSTALLED_BIN"
