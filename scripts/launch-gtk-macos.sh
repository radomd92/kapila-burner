#!/bin/bash
set -euo pipefail

# This file is installed as launch-gtk.command beside the native executable.
bundle=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
export PATH="$bundle/tools/bin:/usr/bin:/bin:/usr/sbin:/sbin"
export XDG_DATA_DIRS="$bundle/share:/usr/share"
export GTK_DATA_PREFIX="$bundle"
export GTK_EXE_PREFIX="$bundle"
export GSETTINGS_SCHEMA_DIR="$bundle/share/glib-2.0/schemas"
export GDK_PIXBUF_MODULEDIR="$bundle/lib/gdk-pixbuf-2.0/2.10.0/loaders"
export FONTCONFIG_PATH="$bundle/etc/fonts"
export FONTCONFIG_FILE="$bundle/etc/fonts/fonts.conf"
# Do not inherit a developer machine's overrides or Homebrew search paths.
unset DYLD_LIBRARY_PATH DYLD_FALLBACK_LIBRARY_PATH DYLD_FRAMEWORK_PATH
unset DYLD_FALLBACK_FRAMEWORK_PATH DYLD_INSERT_LIBRARIES GTK_PATH GTK_MODULES
unset GIO_EXTRA_MODULES GDK_PIXBUF_MODULE_FILE
unset GTK_IM_MODULE_FILE GTK_IM_MODULE
unset FONTCONFIG_SYSROOT
export GTK_PATH="$bundle/lib/gtk-3.0"
export GIO_MODULE_DIR="$bundle/lib/gio/modules"
cache_base=${XDG_CACHE_HOME:-"$HOME/Library/Caches"}
mkdir -p "$cache_base/Kapila Burner"
cache=$(mktemp -d "$cache_base/Kapila Burner/runtime.XXXXXXXX")
trap 'rm -rf -- "$cache"' EXIT
export GDK_PIXBUF_MODULE_FILE="$cache/loaders.cache"
"$bundle/tools/bin/gdk-pixbuf-query-loaders" > "$GDK_PIXBUF_MODULE_FILE"
if [[ -x "$bundle/tools/bin/gtk-query-immodules-3.0" ]]; then
  export GTK_IM_MODULE_FILE="$cache/immodules.cache"
  shopt -s nullglob
  immodules=("$bundle"/lib/gtk-3.0/*/immodules/*.so "$bundle"/lib/gtk-3.0/*/immodules/*.dylib)
  if ((${#immodules[@]})); then
    "$bundle/tools/bin/gtk-query-immodules-3.0" "${immodules[@]}" > "$GTK_IM_MODULE_FILE"
  else
    unset GTK_IM_MODULE_FILE
  fi
  shopt -u nullglob
fi
export XDG_CACHE_HOME="$cache_base"
output="$HOME/Library/Application Support/Kapila Burner/output"
mkdir -p "$output"
# The application locates its utilities relative to its working directory.
cd -- "$bundle"
# User-supplied flags come last, so explicit -root and -output override defaults.
"$bundle/kapila-burner" -root "$HOME/Music" -output "$output" "$@"
