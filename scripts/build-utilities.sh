#!/usr/bin/env bash
# Native prerequisites:
# macOS Intel: Xcode CLI tools; brew install bash make autoconf automake libtool
#              pkg-config libpng zlib libcdio popt
# Windows: MSYS2 MINGW64 shell, MSYS bash/make/autoconf/automake/libtool/patch,
#          mingw-w64-x86_64-{gcc,pkgconf,libpng,zlib,libcdio,popt}.
# Both: curl, tar (xz support), a SHA-256 tool, flex, bison, Perl.
# Linux is also supported for native verification with the equivalent -dev packages.
# The packaging layer owns non-system DLL/dylib closure and its corresponding sources.
# Offline rebuild: run the distributed SOURCE-ARCHIVES/build-utilities.sh with
# UTILITY_OFFLINE=1. UTILITY_SOURCE_DIR can override its sibling archive directory.
set -euo pipefail
export LC_ALL=C
scripts=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
if [[ $# != 1 || -z $1 ]]; then
    printf 'Usage: bash scripts/build-utilities.sh STAGE\n' >&2
    exit 2
fi
case $(uname -s) in
    MINGW64*|MSYS*|MINGW_NT*)
        [[ ${MSYSTEM:-} == MINGW64 ]] || { printf 'Use the MSYS2 MINGW64 shell.\n' >&2; exit 1; }
        platform=WindowsMINGW64; exe=.exe; compiler=gcc ;;
    Darwin)
        [[ $(uname -m) == x86_64 ]] || { printf 'A native Intel macOS build is required.\n' >&2; exit 1; }
        platform=macOSIntel; exe=; compiler=clang
        for dep in libpng zlib libcdio popt; do
            prefix=$(brew --prefix "$dep")
            export PKG_CONFIG_PATH="$prefix/lib/pkgconfig${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"
        done ;;
    Linux) platform=Linux; exe=; compiler=gcc ;;
    *) printf 'Unsupported native platform\n' >&2; exit 1 ;;
esac
make=${MAKE:-make}
if [[ $platform == macOSIntel ]]; then make=${MAKE:-gmake}; fi
for tool in "$make" "$compiler" curl tar patch autoconf autoheader pkg-config; do
    command -v "$tool" >/dev/null || { printf 'Missing prerequisite: %s\n' "$tool" >&2; exit 1; }
done
triplet=$("$compiler" -dumpmachine)
pkg-config --exists libpng zlib libcdio libiso9660 popt
if command -v sha256sum >/dev/null; then
    sha256() { sha256sum "$1"; }
else
    sha256() { shasum -a 256 "$1"; }
fi
mkdir -p "$1"
stage=$(cd "$1" && pwd)
work=${UTILITY_BUILD_ROOT:-${TMPDIR:-/tmp}/dvd-audio-utilities-$platform}
mkdir -p "$work"
work=$(cd "$work" && pwd)
# Legacy makefiles interpolate paths without quoting. Fail early rather than
# silently mixing MSYS headers with native headers or building in a broken path.
if [[ $work == *[[:space:]]* || $stage == *[[:space:]]* ]]; then
    printf 'Utility build and staging paths must not contain whitespace.\n' >&2
    exit 1
fi
jobs=${UTILITY_JOBS:-$(getconf _NPROCESSORS_ONLN 2>/dev/null || printf 2)}
[[ $jobs =~ ^[1-9][0-9]*$ ]] || { printf 'Invalid UTILITY_JOBS\n' >&2; exit 1; }
mkdir -p "$work/downloads" "$work/src" "$stage/tools/bin" \
    "$stage/tools/share/dvda-author" "$stage/tools/share/licenses" "$stage/SOURCE-ARCHIVES"
exec > >(tee "$work/build.log") 2>&1
trap 'printf "Utility build failed at line %s. Log: %s/build.log\n" "$LINENO" "$work" >&2' ERR
printf 'Building native utilities: %s, %s jobs\n' "$platform" "$jobs"
source_dir=$(cd "${UTILITY_SOURCE_DIR:-$scripts}" && pwd)
manifest=$(mktemp "$work/distribution-manifest.XXXXXX")
printf '# name archive sha256 URL license\n' > "$manifest"
printf '# dvda-author is normalized original source, without unused vendored dependencies or binary payloads.\n' >> "$manifest"
printf '# Original upstream URL/SHA-256: utility-sources.upstream.tsv. Local archives support offline rebuilding.\n' >> "$manifest"
while read -r name archive digest url license; do
    [[ -n $name && $name != \#* ]] || continue
    file="$work/downloads/$archive"
    if [[ -f $source_dir/$archive ]]; then file="$source_dir/$archive"; fi
    if [[ ! -f $file ]]; then
        if [[ ${UTILITY_OFFLINE:-0} == 1 || $url == local:* ]]; then
            printf 'Missing local source archive: %s (source directory: %s)\n' "$archive" "$source_dir" >&2
            exit 1
        fi
        curl --fail --location --retry 3 --output "$file.part" "$url"
        mv "$file.part" "$file"
    fi
    actual=$(sha256 "$file"); actual=${actual%% *}
    [[ $actual == "$digest" ]] || { printf 'SHA-256 mismatch: %s\n' "$archive" >&2; exit 1; }
    if [[ $name == dvda-author && $archive != dvda-author-source.tar.gz ]]; then
        # Always curate from the verified, unmodified upstream archive, never
        # from a configured/patched build tree. Keep the full download in cache.
        pristine=$(mktemp -d "$work/dvda-pristine.XXXXXX")
        mkdir "$pristine/dvda-author-source"
        tar -tf "$file" > "$pristine/upstream-members"
        while IFS= read -r member; do
            relative=${member#*/}
            case "$relative" in
                ''|*/) continue ;;
                src/*|libutils/*|libfixwav/*|libiberty/*|m4/*|m4.extra/*|m4.extra.dvdauthor/*|mk/*|images/*) ;;
                */*) continue ;;
            esac
            printf '%s\n' "$member"
        done < "$pristine/upstream-members" > "$pristine/selected-members"
        tar -xf "$file" -C "$pristine/dvda-author-source" --strip-components=1 \
            -T "$pristine/selected-members"
        # Explicit sorted file order works with GNU tar and macOS bsdtar.
        while IFS= read -r member; do
            printf 'dvda-author-source/%s\n' "${member#*/}"
        done < "$pristine/selected-members" | sort > "$pristine/normalized-members"
        tarflags=(--format=ustar)
        case $(tar --version) in
            *'GNU tar'*) tarflags+=(--mtime=@0 --owner=0 --group=0 --numeric-owner) ;;
            *bsdtar*) tarflags+=(--uid=0 --gid=0 --uname=root --gname=root) ;;
        esac
        tar "${tarflags[@]}" -cf - -C "$pristine" -T "$pristine/normalized-members" \
            | gzip -n > "$work/downloads/dvda-author-source.tar.gz.part"
        mv "$work/downloads/dvda-author-source.tar.gz.part" "$work/downloads/dvda-author-source.tar.gz"
        rm -rf "$pristine"
        rm -f "$stage/SOURCE-ARCHIVES/$archive"
        archive=dvda-author-source.tar.gz
        file="$work/downloads/$archive"
        digest=$(sha256 "$file"); digest=${digest%% *}
        url="local:$archive"
    fi
    if [[ ! -f $work/src/$name/.unpacked ]]; then
        mkdir -p "$work/src/$name"
        tar -xf "$file" -C "$work/src/$name" --strip-components=1
        touch "$work/src/$name/.unpacked"
    fi
    if [[ $file != "$stage/SOURCE-ARCHIVES/$archive" ]]; then
        cp "$file" "$stage/SOURCE-ARCHIVES/$archive"
    fi
    printf '%s %s %s %s %s\n' "$name" "$archive" "$digest" "$url" "$license" >> "$manifest"
done < "$scripts/utility-sources.tsv"
upstream="$scripts/utility-sources.tsv"
if [[ -f $scripts/utility-sources.upstream.tsv ]]; then upstream="$scripts/utility-sources.upstream.tsv"; fi
if [[ $upstream != "$stage/SOURCE-ARCHIVES/utility-sources.upstream.tsv" ]]; then
    cp "$upstream" "$stage/SOURCE-ARCHIVES/utility-sources.upstream.tsv"
fi
mv "$manifest" "$stage/SOURCE-ARCHIVES/utility-sources.tsv"
if [[ $scripts != "$stage/SOURCE-ARCHIVES" ]]; then
    cp "$scripts/build-utilities.sh" "$stage/SOURCE-ARCHIVES/"
    cp -R "$scripts/patches" "$stage/SOURCE-ARCHIVES/"
fi
dvda="$work/src/dvda-author"
avprefix="$dvda/local"
(
    cd "$dvda"
    if [[ ! -f .utility-patched ]]; then
        patch -p1 < "$scripts/patches/dvda-author-core.patch"
        touch .utility-patched
    fi
    if command -v glibtoolize >/dev/null; then glibtoolize --copy --force --install
    else libtoolize --copy --force --install; fi
    autoconf -f -I m4 -I m4.extra -I m4.extra.dvdauthor
    autoheader
    # Upstream configure deletes local/, so configure BEFORE installing FFmpeg.
    CC="$compiler" CFLAGS="${CFLAGS:--O2} $(pkg-config --cflags libpng)" \
        ./configure --enable-core-build --prefix="$stage/tools" --build="$triplet"
)
(
    cd "$work/src/ffmpeg"
    ./configure --prefix="$avprefix" --cc="$compiler" \
        --disable-autodetect --disable-network --disable-doc --disable-debug \
        --disable-programs --disable-x86asm --disable-inline-asm --disable-alsa \
        --disable-devices --enable-indev=lavfi --disable-shared --enable-static
    "$make" -j"$jobs"
    "$make" install
)
cp "$work/src/ffmpeg/COPYING.LGPLv2.1" "$stage/tools/share/licenses/FFmpeg-private-4.2.4.txt"
(
    cd "$work/src/ffmpeg-modern"
    ./configure --prefix="$work/ffmpeg-modern-install" --cc="$compiler" \
        --disable-autodetect --disable-network --disable-doc --disable-debug \
        --disable-ffplay --disable-x86asm --disable-inline-asm --disable-alsa \
        --disable-devices --enable-indev=lavfi --disable-shared --enable-static
    "$make" -j"$jobs"
    "$make" install
)
cp "$work/ffmpeg-modern-install/bin/ffmpeg$exe" "$work/ffmpeg-modern-install/bin/ffprobe$exe" "$stage/tools/bin/"
cp "$work/src/ffmpeg-modern/COPYING.LGPLv2.1" "$stage/tools/share/licenses/FFmpeg.txt"
(
    cd "$dvda"
    # Build in dependency order. Upstream's parallel all target races on .a files
    # and .ONESHELL can hide a failed recursive make or link behind a final echo.
    export CPPFLAGS="${CPPFLAGS:-} -DHAVE_CONFIG_H"
    export CFLAGS="${CFLAGS:--O2} -std=gnu99"
    "$make" -B -C libutils/src CC="$compiler"
    "$make" -B -C libfixwav/src CC="$compiler"
    liberty=
    winlibs=
    if [[ $platform == WindowsMINGW64 ]]; then
        "$make" -B -C libiberty/src CC="$compiler"
        liberty="$dvda/libiberty/src/libiberty.a"
        winlibs='-lwinmm -lbcrypt'
    fi
    avlibs=$(PKG_CONFIG_PATH="$avprefix/lib/pkgconfig" pkg-config --static --libs libavfilter libavformat libavcodec libswscale libswresample libavutil)
    "$make" -B -C src -j"$jobs" CC="$compiler" \
        LDLIBS="$dvda/libutils/src/libc_utils.a $dvda/libfixwav/src/libfixwav.a $liberty $avlibs $winlibs -lm" \
        LDFLAGS="${LDFLAGS:-}" dvda-author
    cp "src/dvda-author$exe" "$stage/tools/bin/dvda-author$exe"
)
cp "$dvda/COPYING" "$stage/tools/share/licenses/dvda-author.txt"
(
    cd "$work/src/vcdimager"
    CC="$compiler" CFLAGS="${CFLAGS:--O2} -std=gnu99 -Wno-error=implicit-function-declaration -Wno-error=incompatible-pointer-types" \
        ./configure --prefix="$work/vcd-install" --disable-shared --enable-static \
        --without-xml-frontend --build="$triplet" --host="$triplet"
    "$make" -C lib -j"$jobs"
    "$make" -C frontends/cli -j"$jobs"
    cp "frontends/cli/vcdimager$exe" "$stage/tools/bin/"
)
cp "$work/src/vcdimager/COPYING" "$stage/tools/share/licenses/VCDImager.txt"
(
    cd "$work/src/schilytools"
    # Only mkisofs and its internal static libraries, not recording utilities.
    # Configuration must finish before the library sub-builds start.
    "$make" -C inc CCOM="$compiler" COPTX=-std=gnu99 GMAKE_NOWARN=true
    for dir in libschily libdeflt libmdigest libfind libfile libhfs_iso libsiconv libcdrdeflt libscg librscg libscgcmd; do
        "$make" -C "$dir" -j"$jobs" CCOM="$compiler" COPTX=-std=gnu99 GMAKE_NOWARN=true
    done
    "$make" -C mkisofs -j"$jobs" CCOM="$compiler" COPTX=-std=gnu99 GMAKE_NOWARN=true
    binaries=(mkisofs/OBJ/*/mkisofs"$exe")
    [[ ${#binaries[@]} == 1 && -f ${binaries[0]} ]]
    cp "${binaries[0]}" "$stage/tools/bin/"
)
cp "$work/src/schilytools/mkisofs/COPYING" "$stage/tools/share/licenses/mkisofs.txt"
cp "$work/src/schilytools/CDDL.Schily.txt" "$stage/tools/share/licenses/Schily-CDDL.txt"
bin="$stage/tools/bin"
smoke=$(mktemp -d "$work/smoke.XXXXXX")
mkdir -p "$smoke/audio" "$smoke/lplex" "$smoke/lplex-output" "$smoke/DVD"
"$bin/ffmpeg$exe" -nostdin -v error -f lavfi -i sine=frequency=440:duration=2 \
    -ar 48000 -ac 2 -c:a pcm_s16le -rf64 never "$smoke/audio.wav"
"$bin/ffprobe$exe" -v error -show_entries stream=codec_name,sample_rate,channels \
    -of default=noprint_wrappers=1 "$smoke/audio.wav"
"$bin/dvda-author$exe" -W --bindir "$bin" --datadir "$stage/tools/share/dvda-author" \
    --workdir "$smoke" -D "$smoke/audio" --lplex-tempdir "$smoke/lplex" \
    --lplex-output "$smoke/lplex-output" -P0 -o "$smoke/DVD" -g "$smoke/audio.wav"
test -s "$smoke/DVD/AUDIO_TS/AUDIO_TS.IFO"
test -s "$smoke/DVD/AUDIO_TS/ATS_01_1.AOB"
"$bin/mkisofs$exe" -dvd-audio -udf -o "$smoke/DVD.iso" "$smoke/DVD"
test -s "$smoke/DVD.iso"
"$bin/ffmpeg$exe" -nostdin -v error -f lavfi -i color=c=black:s=352x288:r=25:d=2 \
    -i "$smoke/audio.wav" -map 0:V:0 -map 1:a:0 \
    -target pal-vcd -vf 'scale=352:288,tpad=stop_mode=clone:stop_duration=1' \
    -af apad -t 2 "$smoke/vcd.mpg"
"$bin/vcdimager$exe" --type=vcd2 --bin-file="$smoke/vcd.bin" \
    --cue-file="$smoke/vcd.cue" "$smoke/vcd.mpg"
test -s "$smoke/vcd.bin"
test -s "$smoke/vcd.cue"
{
    printf 'Platform: %s\nCompiler: %s\n' "$platform" "$compiler"
    "$compiler" --version
    printf '\nNative dependency versions (packaging must retain their sources):\n'
    for dep in libpng zlib libcdio libiso9660 popt; do
        printf '%s: %s\n' "$dep" "$(pkg-config --modversion "$dep")"
    done
    printf '\nPublic FFmpeg 9.0.2 configuration (user media input):\n'
    "$bin/ffmpeg$exe" -version
    "$bin/ffprobe$exe" -version
    "$bin/vcdimager$exe" --version
    "$bin/mkisofs$exe" -version
    printf '\nAuthor configured with --enable-core-build; private static FFmpeg 4.2.4 (--disable-programs).\n'
    printf 'Legacy FFmpeg executables are neither built nor shipped; public ffmpeg/ffprobe use 9.0.2.\n'
    printf 'Use -W and existing --bindir, --datadir, --workdir, -D, --lplex-tempdir, --lplex-output paths.\n'
    printf 'Core author accepts PCM WAV; application FFmpeg handles other inputs.\n'
    printf 'Source archives, hashes, patches and rebuild script: SOURCE-ARCHIVES/.\n'
    printf 'Offline rebuild: UTILITY_OFFLINE=1 bash SOURCE-ARCHIVES/build-utilities.sh NEW_STAGE\n'
    printf 'UTILITY_SOURCE_DIR overrides the archive lookup directory. Native prerequisites are still required.\n'
    printf 'dvda-author-source.tar.gz contains unpatched original core sources and build inputs;\n'
    printf 'unused vendored dependencies and binary payloads are omitted. Upstream provenance: utility-sources.upstream.tsv.\n'
    printf 'DVD-Audio tree + true -dvd-audio ISO and PAL VCD BIN/CUE smoke: PASS\n'
} > "$stage/BUILD-UTILITIES.txt" 2>&1
printf 'Utility smoke PASS: %s\n' "$smoke"
