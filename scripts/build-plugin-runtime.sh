#!/usr/bin/env sh
# Builds the runtime binaries the plugin ships.
#
# The plugin carries one binary per supported platform so installing the
# plugin is the whole installation: no download, no checksum, no consent
# prompt for fetching code from the internet. The cost is that these binaries
# are committed, so they can go stale - CI rebuilds them and compares byte for
# byte, which is why the build has to be reproducible.
set -eu

root="$(cd "$(dirname "$0")/.." && pwd)"
out="${1:-$root/plugins/prayops/runtime}"

version="$(sed -n 's/.*"runtimeVersion"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
  "$root/plugins/prayops/runtime-manifest.json")"
if [ -z "$version" ]; then
  echo "build-plugin-runtime: no runtimeVersion in the manifest" >&2
  exit 1
fi

for target in darwin_arm64 darwin_amd64 linux_arm64 linux_amd64; do
  os="${target%_*}"
  arch="${target#*_}"

  mkdir -p "$out/$target"
  # -trimpath and -s -w remove the build directory and the symbol table, the
  # two things that would otherwise differ between one machine and the next.
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags "-s -w -X main.version=$version" \
    -o "$out/$target/prayops" "$root/cmd/prayops"

  echo "built $target v$version"
done
