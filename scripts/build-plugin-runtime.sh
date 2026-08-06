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

# go.mod's toolchain line is a floor, not a pin: a machine with a newer patch
# release uses that instead, and the bytes come out different. GOTOOLCHAIN
# names the exact one, downloading it when it is not already there.
toolchain="$(sed -n 's/^toolchain[[:space:]]*\(go[0-9.]*\).*/\1/p' "$root/go.mod")"
if [ -z "$toolchain" ]; then
  echo "build-plugin-runtime: go.mod pins no toolchain" >&2
  exit 1
fi
export GOTOOLCHAIN="$toolchain"

for target in darwin_arm64 darwin_amd64 linux_arm64 linux_amd64; do
  os="${target%_*}"
  arch="${target#*_}"

  mkdir -p "$out/$target"
  # -trimpath drops the build directory and -s -w the symbol table, the two
  # things that differ between one machine and the next.
  #
  # -buildvcs=false matters more than either. Go stamps the commit hash and
  # its timestamp into the binary, so committing a binary changes the hash the
  # next build would stamp - a byte comparison could never pass, no matter how
  # faithfully the source was rebuilt.
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -buildvcs=false -ldflags "-s -w -X main.version=$version" \
    -o "$out/$target/prayops" "$root/cmd/prayops"

  echo "built $target v$version"
done
