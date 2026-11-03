#!/bin/sh
# Runs OmniFlow from source. The binary is signed with a stable identity so
# macOS keeps permission grants across rebuilds.
set -eu
cd "$(dirname "$0")/.."
. scripts/signing.sh

identity="$(mygo_sign_identity)"
mkdir -p out
go build -o out/omniflow ./cmd/omniflow

if [ -n "$identity" ] && command -v codesign >/dev/null 2>&1; then
  codesign --force --sign "$identity" --timestamp=none out/omniflow
elif [ "$(uname -s)" = Darwin ]; then
  echo "warning: no Apple Development identity found; OmniFlow is unsigned," >&2
  echo "         so macOS asks for permissions again after every rebuild." >&2
fi

exec ./out/omniflow "$@"
