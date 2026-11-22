#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
: "${GH_TOKEN:?GitHub release token is required}"
: "${GITHUB_SHA:?GitHub commit is required}"
: "${GITHUB_REF:?GitHub ref is required}"
assets=(
  kapila-burner-linux-amd64.tar.gz
  kapila-burner-windows-amd64.zip
  kapila-burner-macos-amd64.tar.gz
  kapila-burner-windows-amd64-setup.exe
  kapila-burner-macos-amd64.dmg
  kapila-burner-windows-amd64-sources.tar.gz
  kapila-burner-macos-amd64-sources.tar.gz
)
for asset in "${assets[@]}"; do
  test -s "dist/$asset"
done
(cd dist && sha256sum "${assets[@]}" > SHA256SUMS)

if [[ $GITHUB_REF == refs/tags/v* ]]; then
  tag=${GITHUB_REF#refs/tags/}
  title="Kapila Burner $tag"
  prerelease=false
else
  tag=continuous
  title='Kapila Burner continuous build'
  prerelease=true
  # Move the rolling tag as well as its assets so source and binaries agree.
  if gh api "repos/{owner}/{repo}/git/ref/tags/$tag" --silent; then
    gh api --method PATCH "repos/{owner}/{repo}/git/refs/tags/$tag" \
      -f sha="$GITHUB_SHA" -F force=true --silent
  else
    gh api --method POST 'repos/{owner}/{repo}/git/refs' \
      -f ref="refs/tags/$tag" -f sha="$GITHUB_SHA" --silent
  fi
fi
notes="Commit: $GITHUB_SHA

Each platform archive contains native GTK and portable web executables.
Windows and macOS include GTK, FFmpeg/FFprobe, VCDImager, dvda-author and mkisofs.
Windows has a per-user installer; macOS has a relocatable .app and DMG.
Linux still requires system GTK and external authoring tools.
Source archives include utility/runtime sources, patches and build records.
macOS is ad-hoc signed, not Developer ID signed or notarized.
Physical burning is Linux-only.
Read README.txt inside the archive before running. Verify downloads with SHA256SUMS.
"
if gh release view "$tag" >/dev/null 2>&1; then
  gh release edit "$tag" --title "$title" --notes "$notes" --prerelease="$prerelease"
else
  gh release create "$tag" --verify-tag --title "$title" --notes "$notes" \
    --prerelease="$prerelease"
fi
paths=()
for asset in "${assets[@]}"; do paths+=("dist/$asset"); done
gh release upload "$tag" "${paths[@]}" dist/SHA256SUMS --clobber
