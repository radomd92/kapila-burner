#!/usr/bin/env bash
set -euo pipefail

# Run inside MINGW64. ldd includes Windows system DLLs, which must not be shipped.
destination=${1:?Usage: bundle-windows.sh package-directory}
[[ ${MSYSTEM:-} == MINGW64 ]]
prefix=/mingw64
queue=("$destination/kapila-burner.exe")

copy_dependencies() {
  local binary dependencies line dependency
  while ((${#queue[@]})); do
    binary=${queue[0]}
    queue=("${queue[@]:1}")
    dependencies=$(ldd "$binary")
    while IFS= read -r line; do
      if [[ $line == *'not found'* ]]; then
        printf 'Unresolved dependency of %s: %s\n' "$binary" "$line" >&2
        exit 1
      fi
      if [[ $line =~ (/mingw64/[^[:space:]]+\.[dD][lL][lL]) ]]; then
        dependency=${BASH_REMATCH[1]}
        if [[ ! -f "$destination/${dependency##*/}" ]]; then
          cp "$dependency" "$destination/"
          queue+=("$dependency")
        fi
      fi
    done <<< "$dependencies"
  done
}

# Loader modules are opened dynamically and do not appear in the executable's ldd.
cp "$prefix/bin/gdk-pixbuf-query-loaders.exe" "$destination/"
queue+=("$prefix/bin/gdk-pixbuf-query-loaders.exe")
mkdir -p "$destination/lib/gdk-pixbuf-2.0/2.10.0" "$destination/share"
cp -R "$prefix/lib/gdk-pixbuf-2.0/2.10.0/loaders" "$destination/lib/gdk-pixbuf-2.0/2.10.0/"
for module in "$prefix"/lib/gdk-pixbuf-2.0/2.10.0/loaders/*.dll; do
  queue+=("$module")
done
copy_dependencies
cp -R "$prefix/share/glib-2.0" "$destination/share/"
cp -R "$prefix/share/gtk-3.0" "$destination/share/"
mkdir -p "$destination/share/icons"
for theme in Adwaita hicolor; do
  if [[ -d $prefix/share/icons/$theme ]]; then
    cp -R "$prefix/share/icons/$theme" "$destination/share/icons/"
  fi
done
# Preserve licensing notices for the redistributed MSYS2 runtime packages.
if [[ -d $prefix/share/licenses ]]; then
  cp -R "$prefix/share/licenses" "$destination/THIRD-PARTY-LICENSES"
fi
