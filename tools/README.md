# Local DVD-Audio Tools

Use `bin/dvda-author` and `bin/mkisofs`, not the package's hard-coded
`opt/dvda-author/dvda-author-dev` launcher. No system installation is needed.

## Verified Commands

```sh
TOOLS=/home/rado/Music/dvd-audio-studio/tools/bin
timeout 15s "$TOOLS/dvda-author" -W -P0 \
  -o /tmp/opencode/dvda-backend-smoke/disc \
  -g /tmp/opencode/dvda-smoke.wav
timeout 15s "$TOOLS/mkisofs" -dvd-audio -V DVDA_SMOKE \
  -o /tmp/opencode/dvda-backend-smoke/disc.iso \
  /tmp/opencode/dvda-backend-smoke/disc
```

Verified with a one-second stereo 24-bit/96-kHz PCM WAV. Both commands exited
zero. Authoring produced six AUDIO_TS files including ATS_01_1.AOB (589824
bytes). The ISO was 1619968 bytes and identified as UDF 1.5, label DVDA_SMOKE.
Hardware playback, menus, MLP, and burning were not tested.

No LD_LIBRARY_PATH or other environment override is needed. The launcher uses
TMPDIR (default /tmp) for unique scratch directories and removes its scratch
tree on exit. Use absolute input and output paths as in the backend contract.
Output directories are created automatically. WARNING: upstream authoring
refreshes/deletes output directory contents; always use a dedicated disc path.

## Launcher Workaround

Upstream make_absolute() calls realpath(path, NULL), so nonexistent output or
temporary directories become NULL. secure_mkdir() recursively retries that
same NULL path forever. The launcher pre-creates directories given through
separate -o/-D arguments or long output/tempdir options, supplies existing
private audio/lplex scratch directories, sets the current working directory,
and supplies local bindir/datadir paths. It disables the default configuration
file with -W. Additional -W from the backend is harmless.

Source inspection: upstream git https://git.code.sf.net/p/dvd-audio/dvda-author,
commit 7f405972a2e88acd4fbbfef1d847d3261791686b;
libutils/src/libc_utils.c (make_absolute and secure_mkdir), and
src/command_line_parsing.c (directory options and directory creation).
The packaged ELF binaries were not modified or rebuilt.

## Provenance And Dependencies

Upstream package: dvda-author-21.10-ubuntu-impish-amd64.deb (amd64).
Downloaded from the DVD audio Tools SourceForge project, using its Netix mirror:

https://sourceforge.net/projects/dvd-audio/files/dvda-author/dvda-author-dev/linux%20%2B%20source%20%28amd64%20build%29/

Archive retained at /tmp/opencode/dvda-author-21.10-ubuntu-impish-amd64.deb.
SHA-256: 4f57832ffeb564fa1c803cc604486f3d02cbf15ef0edd856ad18b6d97f61bd91

Extracted with dpkg-deb -x into this tools directory; no maintainer/install
scripts were executed. dvda reports 2021-dev; mkisofs reports 3.02a09 and
supports -dvd-audio and -dvd-hybrid.

No missing libraries reported by ldd for the two main ELF binaries:
dvda uses libm.so.6, libpng16.so.16, libz.so.1, libc.so.6;
mkisofs uses libc.so.6. Optional auxiliary executables were not exhaustively
dependency-tested. The launchers require standard sh, mkdir, mktemp, rm,
dirname, and pwd utilities.
