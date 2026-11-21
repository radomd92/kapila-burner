#!/usr/bin/env bash
set -euo pipefail
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
ls -ld "$root"
mkdir -p "$root/debs" "$root/sysroot" "$root/cache/tmp"
ls -ld "$root/debs"
cd -- "$root/debs"
mapfile -t packages < "$root/packages.txt"
apt-get download "${packages[@]}"
for deb in ./*.deb; do
    dpkg-deb -x "$deb" "$root/sysroot"
done
# Relocate only extracted metadata; system .pc files retain their real paths.
while IFS= read -r -d '' pc; do
    GTK_LOCAL_PREFIX="$root/sysroot/usr" perl -pi -e 's{^prefix=/usr$}{prefix=$ENV{GTK_LOCAL_PREFIX}}' "$pc"
done < <(find "$root/sysroot" -name '*.pc' -print0)
# Development packages link to runtime sonames supplied by the host packages.
while IFS= read -r -d '' link; do
    if [[ ! -e "$link" ]]; then
        host="$(dirname -- "${link#"$root/sysroot"}")/$(readlink "$link")"
        if [[ -e "$host" ]]; then
            ln -sfn "$(readlink -f "$host")" "$link"
        fi
    fi
done < <(find "$root/sysroot" -type l -name '*.so' -print0)
sha256sum ./*.deb > "$root/debs.sha256"
