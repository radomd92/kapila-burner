#!/bin/bash
set -euo pipefail
[[ $# == 2 ]] || { printf 'Usage: %s STAGE DIST\n' "$0" >&2; exit 1; }
scripts=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
stage=$(cd "$1" && pwd)
output=$(cd "$2" && pwd)
app="$stage/kapila-maker.app"
python3 -B "$scripts/macruntime/make_app.py" "$stage"
# Check every relocated image, including PlugIns, from the common Contents root.
python3 -B "$scripts/macruntime/bundle.py" --verify-only "$app/Contents"
python3 -B "$scripts/smoke-bundle.py" "$app/Contents"
dmg_stage=$(mktemp -d "${TMPDIR:-/tmp}/kapila-dmg.XXXXXXXX")
trap 'rm -rf -- "$dmg_stage"' EXIT
ditto "$app" "$dmg_stage/kapila-maker.app"
cp "$stage/README.txt" "$dmg_stage/README.txt"
ln -s /Applications "$dmg_stage/Applications"
hdiutil create -volname 'kapila-maker' -srcfolder "$dmg_stage" -ov -format UDZO \
  "$output/kapila-maker-macos-amd64.dmg"
