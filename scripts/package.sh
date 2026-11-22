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
name="kapila-maker-$platform"
package="dist/$name"
# Only clean this script's generated, platform-specific staging directory.
rm -rf -- "$package"
mkdir -p "$package"
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$package/kapila-maker-web$suffix" .
export CGO_CFLAGS="${CGO_CFLAGS:--O0 -g0 -Wno-deprecated-declarations}"
# Match make gtk's dependency build flags to reuse the expensive gotk3 cache.
CGO_ENABLED=1 go build -tags 'gtk gtk_3_20' -ldflags='-s -w' \
  -o "$package/kapila-maker$suffix" ./cmd/kapila-maker
"$package/kapila-maker-web$suffix" -h
cp scripts/PACKAGE.txt "$package/README.txt"
commit=${GITHUB_SHA:-$(git rev-parse --verify HEAD 2>/dev/null || printf 'uncommitted')}
printf '\nBuild platform: %s\nGo: %s\nCommit: %s\n' \
  "$platform" "$(go version)" "$commit" >> "$package/README.txt"

case "$os" in
  windows)
    native_python=$(cygpath -u "${PYTHON_NATIVE:?Native Windows Python is required}")
    if [[ -n ${UTILITY_BUILD_ROOT:-} ]]; then
      export UTILITY_BUILD_ROOT="$(cygpath -u "$UTILITY_BUILD_ROOT")"
    fi
    bash scripts/build-utilities.sh "$package"
    bash scripts/bundle-windows.sh "$package"
    bash scripts/collect-runtime-sources.sh "$package"
    tar -czf "dist/$name-sources.tar.gz" -C "$package" SOURCE-ARCHIVES THIRD-PARTY-LICENSES WINDOWS-RUNTIME-PACKAGES.txt WINDOWS-RUNTIME-FILES.tsv BUILD-UTILITIES.txt
    rm -rf -- "$package/SOURCE-ARCHIVES"
    cp scripts/launch-gtk.cmd "$package/launch-gtk.cmd"
    "$native_python" scripts/smoke-bundle.py "$package"
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/build-windows-installer.ps1 \
      -Stage "$(cygpath -w "$package")" -OutputDir "$(cygpath -w dist)"
    (cd dist && zip -qr "$name.zip" "$name")
    ;;
  darwin)
    # Utility build scripts need modern Bash; the system Bash is version 3.2.
    "$(brew --prefix bash)/bin/bash" scripts/build-utilities.sh "$package"
    bash scripts/bundle-macos.sh "$package"
    bash scripts/collect-runtime-sources.sh "$package"
    tar -czf "dist/$name-sources.tar.gz" -C "$package" SOURCE-ARCHIVES THIRD-PARTY-LICENSES MACOS-RUNTIME-MANIFEST.json BUILD-UTILITIES.txt
    rm -rf -- "$package/SOURCE-ARCHIVES"
    python3 scripts/smoke-bundle.py "$package"
    bash scripts/make-macos-app.sh "$package" dist
    tar -czf "dist/$name.tar.gz" -C dist "$name"
    ;;
  linux)
    ldd "$package/kapila-maker" > "$package/GTK-LINKAGE.txt"
    if grep -q 'not found' "$package/GTK-LINKAGE.txt"; then
      printf 'Missing GTK runtime library\n' >&2
      exit 1
    fi
    "$package/kapila-maker" -h
    tar -czf "dist/$name.tar.gz" -C dist "$name"
    ;;
esac
