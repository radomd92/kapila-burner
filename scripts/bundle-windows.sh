#!/usr/bin/env bash
set -euo pipefail

# Run inside MINGW64 after building both applications and the native utilities.
destination=${1:?Usage: bundle-windows.sh STAGE}
[[ ${MSYSTEM:-} == MINGW64 ]] || { printf 'Use the MSYS2 MINGW64 shell.\n' >&2; exit 1; }
destination=$(cd "$destination" && pwd)
prefix=/mingw64
export LC_ALL=C
export PATH="$prefix/bin:/usr/bin:$PATH"
for executable in kapila-burner.exe kapila-burner-web.exe tools/bin/{ffmpeg,ffprobe,dvda-author,mkisofs,vcdimager}.exe; do
  [[ -f $destination/$executable ]] || { printf 'Missing staged executable: %s\n' "$executable" >&2; exit 1; }
done
declare -A packages=() scanned=() dll_hashes=() dll_names=() copied=()
queue=()
manifest="$destination/WINDOWS-RUNTIME-PACKAGES.txt"
files="$destination/WINDOWS-RUNTIME-FILES.tsv"
printf 'SHA256\tPackage\tInstalled version\tMSYS2 source\tStaged path\tOwnership path\n' > "$files"

copy_runtime() {
  local source=$1 target=$2 owner owners version digest existing key ownership=$1
  [[ -z ${copied["$source|$target"]:-} ]] || return 0
  if ! owners=$(pacman -Qoq -- "$source" 2>/dev/null); then
    # Hook-generated GTK caches belong to the packages owning their directory.
    case ${source##*/} in
      gschemas.compiled|icon-theme.cache|giomodule.cache|immodules.cache)
        ownership=${source%/*}
        owners=$(pacman -Qoq -- "$ownership") ;;
      *) printf 'No MSYS2 package owns runtime file: %s\n' "$source" >&2; exit 1 ;;
    esac
  fi
  digest=$(sha256sum "$source"); digest=${digest%% *}
  if [[ ${source,,} == *.dll ]]; then
    key=${target##*/}; key=${key,,}
    if [[ -n ${dll_hashes[$key]:-} && ${dll_hashes[$key]} != "$digest" ]]; then
      printf 'Conflicting DLL filename: %s (%s)\n' "$target" "$source" >&2
      exit 1
    fi
    dll_hashes[$key]=$digest
    if [[ -n ${dll_names[$key]:-} && ${target%/*} == "$destination" ]]; then
      target="$destination/${dll_names[$key]}"
    fi
  fi
  if [[ -f $target ]]; then
    existing=$(sha256sum "$target"); existing=${existing%% *}
    [[ $existing == "$digest" ]] || { printf 'Runtime file conflict: %s\n' "$target" >&2; exit 1; }
  else
    mkdir -p "${target%/*}"
    cp "$source" "$target"
  fi
  while IFS= read -r owner; do
    version=${packages[$owner]:-}
    if [[ -z $version ]]; then
      version=$(pacman -Q "$owner")
      packages["$owner"]=$version
    fi
    printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$digest" "$owner" "${version#* }" "$source" "${target#"$destination/"}" "$ownership" >> "$files"
  done <<< "$owners"
  copied["$source|$target"]=1
}

copy_tree() {
  local source=$1 target=$2 file
  [[ -d $source ]] || { printf 'Missing GTK runtime directory: %s\n' "$source" >&2; exit 1; }
  while IFS= read -r -d '' file; do
    # The runtime bootstrap regenerates this cache with relocated module paths.
    if [[ $file == "$prefix/lib/gtk-3.0/"*/immodules.cache ]]; then continue; fi
    copy_runtime "$file" "$target/${file#"$source/"}"
  done < <(find -L "$source" -type f -print0)
}

# Preserve existing root DLLs, including case-insensitive Windows filename checks.
while IFS= read -r -d '' file; do
  name=${file##*/}; key=${name,,}
  digest=$(sha256sum "$file"); digest=${digest%% *}
  if [[ -n ${dll_hashes[$key]:-} && ${dll_hashes[$key]} != "$digest" ]]; then
    printf 'Conflicting staged DLL filename: %s\n' "$file" >&2; exit 1
  fi
  dll_hashes[$key]=$digest
  dll_names[$key]=$name
  if [[ -f $prefix/bin/$name ]]; then copy_runtime "$prefix/bin/$name" "$file"; fi
done < <(find "$destination" -maxdepth 1 -type f -iname '*.dll' -print0)

# These modules are loaded dynamically, so executable imports alone miss them.
copy_runtime "$prefix/bin/gdk-pixbuf-query-loaders.exe" "$destination/gdk-pixbuf-query-loaders.exe"
copy_tree "$prefix/lib/gdk-pixbuf-2.0/2.10.0/loaders" "$destination/lib/gdk-pixbuf-2.0/2.10.0/loaders"
for directory in lib/gio/modules lib/gtk-3.0; do
  if [[ -d $prefix/$directory ]]; then copy_tree "$prefix/$directory" "$destination/$directory"; fi
done
immodules=()
if [[ -d $destination/lib/gtk-3.0 ]]; then
  while IFS= read -r -d '' file; do
    immodules+=("$(cygpath -am "$file")")
  done < <(find "$destination/lib/gtk-3.0" -type f -path '*/immodules/*' -iname '*.dll' -print0)
  while IFS= read -r -d '' file; do rm -- "$file"; done < <(find "$destination/lib/gtk-3.0" -type f -name immodules.cache -print0)
fi
if ((${#immodules[@]})); then
  imquery="$prefix/bin/gtk-query-immodules-3.0.exe"
  if [[ ! -f $imquery ]]; then
    imquery=
    for directory in "$prefix/libexec" "$prefix/lib/gtk-3.0"; do
      [[ -d $directory ]] || continue
      while IFS= read -r -d '' file; do imquery=$file; break; done < <(find "$directory" -type f -name gtk-query-immodules-3.0.exe -print0)
      [[ -z $imquery ]] || break
    done
  fi
  [[ -n $imquery && -f $imquery ]] || {
    printf 'GTK input-method modules require gtk-query-immodules-3.0.exe in /mingw64/bin or a GTK libexec directory.\n' >&2; exit 1
  }
  copy_runtime "$imquery" "$destination/gtk-query-immodules-3.0.exe"
fi
for directory in share/glib-2.0 share/gtk-3.0; do
  copy_tree "$prefix/$directory" "$destination/$directory"
done
[[ -f $destination/share/glib-2.0/schemas/gschemas.compiled ]] || {
  printf 'Missing compiled GTK schemas; run glib-compile-schemas /mingw64/share/glib-2.0/schemas first.\n' >&2; exit 1
}
for directory in share/icons/Adwaita share/icons/hicolor etc/fonts share/fontconfig; do
  if [[ -d $prefix/$directory ]]; then copy_tree "$prefix/$directory" "$destination/$directory"; fi
done
# Retain the broad license set as well as the exact per-file ownership manifest.
if [[ -d $prefix/share/licenses ]]; then
  copy_tree "$prefix/share/licenses" "$destination/THIRD-PARTY-LICENSES"
fi

while IFS= read -r -d '' file; do queue+=("$file"); done < <(find "$destination" -type f \( -iname '*.exe' -o -iname '*.dll' \) -print0)
windows=$(cygpath -u "${WINDIR:-C:\\Windows}")
dependency_pattern='(/.*\.[dD][lL][lL])[[:space:]]+\('
for ((index=0; index<${#queue[@]}; index++)); do
  binary=${queue[index]}
  [[ -z ${scanned[$binary]:-} ]] || continue
  scanned[$binary]=1
  dependencies=$(ldd "$binary")
  while IFS= read -r line; do
    if [[ $line == *'not found'* ]]; then
      printf 'Unresolved dependency of %s: %s\n' "$binary" "$line" >&2; exit 1
    fi
    if [[ $line =~ $dependency_pattern ]]; then
      dependency=${BASH_REMATCH[1]}
      case "$dependency" in
        "$prefix"/*)
          copy_runtime "$dependency" "$destination/${dependency##*/}"
          queue+=("$dependency") ;;
        "$destination"/*) queue+=("$dependency") ;;
        *)
          if [[ ${dependency,,} != "${windows,,}/"* ]]; then
            printf 'Non-redistributable dependency of %s: %s\n' "$binary" "$dependency" >&2; exit 1
          fi ;;
      esac
    fi
  done <<< "$dependencies"
done
printf '%s\n' "${packages[@]}" | sort -u > "$manifest"

# Cache paths are build-host inputs only; manifests contain archive names/hashes.
cache=${WINDOWS_PACKAGE_CACHE:-/var/cache/pacman/pkg}
archives="$destination/WINDOWS-RUNTIME-ARCHIVES.txt"
: > "$archives"
if [[ -d $cache ]]; then
  while IFS= read -r -d '' archive; do
    version=$(pacman -Qp "$archive" 2>/dev/null) || continue
    owner=${version%% *}
    if [[ ${packages[$owner]:-} == "$version" ]]; then
      digest=$(sha256sum "$archive"); digest=${digest%% *}
      printf '%s  %s\n' "$digest" "${archive##*/}" >> "$archives"
    fi
  done < <(find "$cache" -maxdepth 1 -type f -name '*.pkg.tar.*' ! -name '*.sig' -print0)
fi
# Optional parent-owned source collector: STAGE, exact package list, full cache path.
if [[ -n ${WINDOWS_COLLECT_SOURCES:-} ]]; then
  cache=$(cd "$cache" && pwd)
  bash "$WINDOWS_COLLECT_SOURCES" "$destination" "$manifest" "$cache"
fi

# Verify the staged closure again without resolving DLLs from /mingw64/bin.
export PATH="$destination:$destination/tools/bin:/usr/bin:$windows/System32"
while IFS= read -r -d '' binary; do
  dependencies=$(ldd "$binary")
  while IFS= read -r line; do
    if [[ $line == *'not found'* ]]; then
      printf 'Unresolved staged dependency of %s: %s\n' "$binary" "$line" >&2; exit 1
    fi
    if [[ $line =~ $dependency_pattern ]]; then
      dependency=${BASH_REMATCH[1]}
      if [[ ${dependency,,} != "${destination,,}/"* && ${dependency,,} != "${windows,,}/"* ]]; then
        printf 'Dependency escapes runtime stage: %s: %s\n' "$binary" "$dependency" >&2; exit 1
      fi
    fi
  done <<< "$dependencies"
done < <(find "$destination" -type f \( -iname '*.exe' -o -iname '*.dll' \) -print0)

# Native smoke commands inherit only the stage and Windows system DLL paths.
export PATH="$destination:$destination/tools/bin:$windows/System32"
if ((${#immodules[@]})); then
  # Explicit native paths prevent GTK from querying its compiled-in build prefix.
  imcache=$(MSYS2_ARG_CONV_EXCL='*' "$destination/gtk-query-immodules-3.0.exe" "${immodules[@]}")
  if [[ ${imcache,,} == *mingw64* ]]; then
    printf 'GTK input-method query cache retains a MinGW build-host path.\n' >&2; exit 1
  fi
  for module in "${immodules[@]}"; do
    [[ $imcache == *\""$module"\"* ]] || {
      printf 'GTK input-method query omitted staged module: %s\n' "$module" >&2; exit 1
    }
  done
  module_pattern='^"([^"]+\.[dD][lL][lL])"$'
  while IFS= read -r line; do
    line=${line%$'\r'}
    if [[ $line =~ $module_pattern ]]; then
      module=${BASH_REMATCH[1]}
      found=false
      for staged_module in "${immodules[@]}"; do
        if [[ $module == "$staged_module" ]]; then found=true; break; fi
      done
      if [[ $found != true ]]; then
        printf 'GTK input-method query retains a non-staged module path: %s\n' "$module" >&2; exit 1
      fi
    fi
  done <<< "$imcache"
fi
"$destination/kapila-burner.exe" -h
"$destination/kapila-burner-web.exe" -h
"$destination/tools/bin/ffmpeg.exe" -version
"$destination/tools/bin/ffprobe.exe" -version
"$destination/tools/bin/dvda-author.exe" -h
"$destination/tools/bin/mkisofs.exe" -version
"$destination/tools/bin/vcdimager.exe" --version
