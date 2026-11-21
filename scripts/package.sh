#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
os=$(go env GOOS)
arch=$(go env GOARCH)
case "$os" in
  linux) platform="linux-$arch"; suffix= ;;
  windows) platform="windows-$arch"; suffix=.exe ;;
  darwin) platform="macos-$arch"; suffix= ;;
  *) printf 'Unsupported packaging OS: %s\n' "$os" >&2; exit 1 ;;
esac
if [[ -n ${EXPECTED_PLATFORM:-} && $platform != "$EXPECTED_PLATFORM" ]]; then
  printf 'Runner platform %s does not match asset label %s\n' "$platform" "$EXPECTED_PLATFORM" >&2
  exit 1
fi
name="kapila-burner-$platform"
package="dist/$name"
# Only clean this script's generated, platform-specific staging directory.
rm -rf -- "$package"
mkdir -p "$package"
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$package/kapila-burner-web$suffix" .
export CGO_CFLAGS="${CGO_CFLAGS:--O0 -g0 -Wno-deprecated-declarations}"
# Match make gtk's dependency build flags to reuse the expensive gotk3 cache.
CGO_ENABLED=1 go build -tags 'gtk gtk_3_20' -ldflags='-s -w' \
  -o "$package/kapila-burner$suffix" ./cmd/dvd-audio-gtk
"$package/kapila-burner-web$suffix" -h
cp scripts/PACKAGE.txt "$package/README.txt"
commit=${GITHUB_SHA:-$(git rev-parse --verify HEAD 2>/dev/null || printf 'uncommitted')}
printf '\nBuild platform: %s\nGo: %s\nCommit: %s\n' \
  "$platform" "$(go version)" "$commit" >> "$package/README.txt"

case "$os" in
  windows)
    bash scripts/bundle-windows.sh "$package"
    "$package/kapila-burner.exe" -h
    cp scripts/launch-gtk.cmd "$package/launch-gtk.cmd"
    (cd dist && zip -qr "$name.zip" "$name")
    ;;
  darwin)
    # Keep Homebrew install names intact: this archive requires Homebrew GTK.
    otool -L "$package/kapila-burner" > "$package/GTK-LINKAGE.txt"
    "$package/kapila-burner" -h
    tar -czf "dist/$name.tar.gz" -C dist "$name"
    ;;
  linux)
    ldd "$package/kapila-burner" > "$package/GTK-LINKAGE.txt"
    if grep -q 'not found' "$package/GTK-LINKAGE.txt"; then
      printf 'Missing GTK runtime library\n' >&2
      exit 1
    fi
    "$package/kapila-burner" -h
    tar -czf "dist/$name.tar.gz" -C dist "$name"
    ;;
esac
