#!/usr/bin/env bash
# Developer-only cross-build; the distributed application needs no Zig/Go.
set -euo pipefail
cd "$(dirname "$0")/.."
: "${ZIG:?Set ZIG to the absolute path of the Zig executable}"
GO=${GO:-go}
for arch in amd64 arm64; do
  target=x86_64-windows-gnu
  if [ "$arch" = arm64 ]; then target=aarch64-windows-gnu; fi
  CGO_ENABLED=1 GOOS=windows GOARCH="$arch" CC="$ZIG cc -target $target" GO="$GO" ./scripts/build.sh
 done
