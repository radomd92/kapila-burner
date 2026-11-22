#!/bin/bash
set -euo pipefail
[[ $# == 1 ]] || { printf 'Usage: %s DMG\n' "$0" >&2; exit 1; }
[[ $(uname -s) == Darwin ]] || { printf 'This test requires macOS.\n' >&2; exit 1; }
project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
smoke="$project_root/scripts/smoke-bundle.py"
[[ -f $1 && -f $smoke ]] || { printf 'Missing DMG or smoke script.\n' >&2; exit 1; }
dmg=$(CDPATH= cd -- "$(dirname -- "$1")" && printf '%s/%s' "$(pwd -P)" "$(basename -- "$1")")
temporary=$(mktemp -d "${TMPDIR:-/tmp}/kapila-installed.XXXXXXXX")
mountpoint="$temporary/mount"
attach_attempted=0
cleanup() {
    result=$?
    trap - EXIT
    if [[ $attach_attempted == 1 ]]; then
        if ! hdiutil detach "$mountpoint"; then
            # A failed attach may leave only the empty, unmounted directory.
            # rmdir cannot remove a live mount, so never recursively delete one.
            if ! hdiutil detach -force "$mountpoint" && ! rmdir "$mountpoint"; then
                printf 'Detach failed; retaining test directory: %s\n' "$temporary" >&2
                exit 1
            fi
        fi
    fi
    rm -rf -- "$temporary"
    exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir "$mountpoint" "$temporary/installed applications"
# hdiutil mounts only the supplied image file; no optical device is accessed.
attach_attempted=1
hdiutil attach -readonly -nobrowse -mountpoint "$mountpoint" "$dmg"
app="$temporary/installed applications/kapila-maker.app"
ditto "$mountpoint/kapila-maker.app" "$app"
contents="$app/Contents"
for directory in MacOS Helpers Frameworks Resources Resources/share/glib-2.0/schemas Resources/tools/share/dvda-author; do
    [[ -d $contents/$directory ]] || { printf 'Installed app is missing directory: %s\n' "$directory" >&2; exit 1; }
done
for executable in MacOS/kapila-maker MacOS/kapila-maker-web Helpers/ffmpeg Helpers/ffprobe Helpers/dvda-author Helpers/mkisofs Helpers/vcdimager Helpers/gdk-pixbuf-query-loaders; do
    [[ -f $contents/$executable && -x $contents/$executable ]] || { printf 'Installed app is missing executable: %s\n' "$executable" >&2; exit 1; }
done
[[ -f $contents/Resources/share/glib-2.0/schemas/gschemas.compiled ]] || {
    printf 'Installed app is missing compiled GTK schemas.\n' >&2; exit 1
}
codesign --verify --deep --strict "$app"
python3 -B "$smoke" "$contents"
printf 'Installed DMG payload and smoke test PASS; no physical burning.\n'
