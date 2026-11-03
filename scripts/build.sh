#!/bin/sh
# Builds the installers signed with a stable identity, so an installed copy
# keeps its macOS permission grants across upgrades.
set -eu
cd "$(dirname "$0")/.."
. scripts/signing.sh

identity="$(mygo_sign_identity)"
if [ -n "$identity" ]; then
  exec go tool mygo build -skip-dmg -o out -sign "$identity" "$@"
fi

echo "warning: no Apple Development identity found; the app is ad-hoc signed." >&2
exec go tool mygo build -skip-dmg -o out "$@"
