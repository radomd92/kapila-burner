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
Windows bundles GTK. Linux needs system GTK 3; macOS Intel needs Homebrew GTK 3
in /usr/local. macOS binaries are unsigned, not notarized, and are not .app bundles.
External authoring tools are not included. Physical burning is Linux-only.
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
