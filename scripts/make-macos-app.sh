#!/bin/bash
set -euo pipefail
stage=$(cd "$1" && pwd)
output=$(cd "$2" && pwd)
app="$stage/Kapila Burner.app"
macos="$app/Contents/MacOS"
mkdir -p "$macos" "$app/Contents/Resources"
for entry in kapila-burner kapila-burner-web tools lib share etc THIRD-PARTY-LICENSES THIRD-PARTY.txt MACOS-RUNTIME-MANIFEST.json BUILD-UTILITIES.txt; do
  if [[ -e $stage/$entry ]]; then mv "$stage/$entry" "$macos/"; fi
done
cp "$stage/README.txt" "$app/Contents/Resources/README.txt"
cp scripts/macos-Info.plist "$app/Contents/Info.plist"
codesign --force --deep --sign - "$app"
codesign --verify --deep --strict "$app"
ln -s 'Kapila Burner.app/Contents/MacOS/kapila-burner' "$stage/kapila-burner"
ln -s 'Kapila Burner.app/Contents/MacOS/kapila-burner-web' "$stage/kapila-burner-web"
cp scripts/launch-app-macos.sh "$stage/launch-gtk.command"
chmod 755 "$stage/launch-gtk.command"
# Check the relocated app, not just the intermediate flat bundle.
python3 scripts/macruntime/bundle.py --verify-only "$macos"
python3 scripts/smoke-bundle.py "$macos"
dmg_stage=$(mktemp -d "${TMPDIR:-/tmp}/kapila-dmg.XXXXXXXX")
trap 'rm -rf -- "$dmg_stage"' EXIT
cp -R "$app" "$dmg_stage/"
cp "$stage/README.txt" "$dmg_stage/README.txt"
ln -s /Applications "$dmg_stage/Applications"
hdiutil create -volname 'Kapila Burner' -srcfolder "$dmg_stage" -ov -format UDZO \
  "$output/kapila-burner-macos-amd64.dmg"
