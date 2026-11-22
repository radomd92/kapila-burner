# kapila-maker

Local GTK 3 development prerequisites.

Provisioned on Linux Mint 22.1, Ubuntu noble base, amd64, using Go 1.22.2.
Nothing was installed into the system and no sudo was used. All project
overlay artifacts are under this directory. The application module now pins
`github.com/gotk3/gotk3 v0.6.4` and includes an optional native GTK adapter.
This is a host-dependent development overlay, not a standalone
SDK: installed runtime libraries and installed development packages are reused.

This overlay is for local source builds, not public runtime installation.
`make smoke-gtk` checks English, Plateau Malagasy, and French. A local
`kapila-maker` build (the same name in release archives) needs
the native runtime libraries and a display, not these development headers.
Run it from the application directory; it calls the shared core
directly and requires no HTTP server or browser.

## Environment

Run in Bash:

```bash
cd /path/to/kapila-burner
source tools/gtk-dev/env.sh
pkg-config --modversion gtk+-3.0
pkg-config --cflags --libs gtk+-3.0
```

The helper derives these paths from its own directory:

```bash
export GTK_DEV_ROOT=/path/to/kapila-burner/tools/gtk-dev
export PKG_CONFIG_PATH="$GTK_DEV_ROOT/sysroot/usr/lib/x86_64-linux-gnu/pkgconfig:$GTK_DEV_ROOT/sysroot/usr/share/pkgconfig${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"
unset PKG_CONFIG_SYSROOT_DIR
export CGO_ENABLED=1
export GOCACHE="$GTK_DEV_ROOT/cache/go-build"
export GOMODCACHE="$GTK_DEV_ROOT/cache/go-mod"
export GOPATH="$GTK_DEV_ROOT/cache/go"
export GOTMPDIR="$GTK_DEV_ROOT/cache/tmp"
```

Do not use global `--define-prefix` or `PKG_CONFIG_SYSROOT_DIR`: dependencies
such as GLib and Pango live in the host /usr. Only extracted .pc files have
their prefix=/usr relocated. Local development .so symlinks point to the host's
existing runtime sonames. No LD_LIBRARY_PATH or custom CGO linker flags are needed.

From the application directory, `make gtk` and `make test-gtk` automatically
source this helper if `pkg-config --exists gtk+-3.0` fails. When system GTK
development files are available they use those instead. On a typical
Debian/Ubuntu host, the system-install alternative is
`sudo apt install libgtk-3-dev pkg-config build-essential`; this is documentation
only, not something the overlay runs. No sudo is needed to build with the
provisioned overlay or run the application.

The nested `tools/gtk-dev/go.mod` is a module boundary that keeps local caches
and smoke tooling out of the root application's `go test ./...` package scope.
It is not the application module; build the real adapter from the parent app
directory, not from this tools module.

## Reprovision

```bash
bash tools/gtk-dev/provision.sh
```

The script uses apt-get download and dpkg-deb -x, never apt-get install, and
does not run package maintainer scripts. Exact package versions are pinned in
packages.txt; downloaded archives remain in debs/ and hashes in debs.sha256.
The package versions must remain available from the configured apt repositories.
The script requires Bash, Perl, find, coreutils, apt-get, and dpkg-deb.

The nine locally extracted packages are libgtk-3-dev, libatk-bridge2.0-dev,
libatspi2.0-dev, libxtst-dev, libepoxy-dev, libegl1-mesa-dev,
libglvnd-core-dev, libglvnd-dev, and wayland-protocols.

Host dependencies were inspected using apt-cache depends/policy and dpkg-query;
apt-get --simulate --no-install-recommends install libgtk-3-dev identified
the missing dependency closure. Existing host packages include libgtk-3-0t64
3.24.41-4ubuntu1.3, GLib development/runtime 2.80.0-6ubuntu3.10, Pango
development/runtime 1.52.1+ds-1build1, Cairo development/runtime
1.18.0-3build1, GdkPixbuf development/runtime 2.42.10+dfsg-3ubuntu3.3,
and the remaining GTK development dependencies.

## Verification

Run from the application directory after sourcing `tools/gtk-dev/env.sh`:

```bash
gcc -Wall -Wextra tools/gtk-dev/check.c -o tools/gtk-dev/check-gtk $(pkg-config --cflags --libs gtk+-3.0)
tools/gtk-dev/check-gtk
```

Verified: pkg-config reports 3.24.41 with local GTK includes/library search
paths and unchanged host dependency include paths. GCC compilation and linking
succeeded, and execution printed:

```text
GTK 3.24.41: window creation succeeded
```

DISPLAY=:0 was set and xdpyinfo confirmed it is accessible. No Xvfb was
downloaded because an existing display works. These runtime tests are not
headless tests; they initialize GTK and create/destroy a window without showing it.

## Application Build

Run from the parent application directory:

```bash
cd /path/to/kapila-burner
make gtk
make test-gtk
make smoke-gtk
./kapila-maker -root /path/to/media
# Or choose another startup language:
./kapila-maker -root /path/to/media -lang en
./kapila-maker -root /path/to/media -lang fr
```

The local Linux application build has been verified; hosted platform results
must be checked separately in Actions. `make smoke-gtk` checks all three
languages, rebuilds as needed, and runs
with `G_DEBUG=fatal-criticals` on an accessible `DISPLAY`. It exercises widgets
without authoring audio; it does not verify disc burning or hardware playback.
`make test-gtk` runs the desktop package's project-handling tests.

The Makefile uses `CGO_CFLAGS="-O0 -g0 -Wno-deprecated-declarations"` and
`-tags "gtk gtk_3_20"`. The `gtk` tag enables the application's optional
`//go:build gtk` files. The `gtk_3_20` compatibility tag limits gotk3 v0.6.4
to APIs that avoid its upstream GTK 3.22 GDK callback bug (an unimported
`callback` symbol). It does **not** change or downgrade the actual toolkit:
the linked native GTK remains 3.24.41. There is no required `gtk_3_24` tag.

Cold gotk3 builds can take several minutes; the previous short timeouts were
not a reliable build verdict. Build caches remain local under this directory.
The default parent `make web` and `make test` do not enable GTK and need no
GTK C headers, despite the optional dependency in the app's go.mod. The web
build and plain `go test ./...` need no C compiler; `make test` uses `-race`,
which requires CGO and a C compiler on Linux, but not GTK development files.

Both interfaces default to Plateau Malagasy (`plt`), support live English (`en`)
and French (`fr`) switching, and exchange
the same project JSON. GTK accepts `-project /path/to/project.json` and provides
explicit Open/Save with dirty-project protection; it does not autosave edits.
The web honors an explicitly saved language, including English. See the detailed
[English README](../../README.en.md) for core architecture, output locking, persistence,
authoring dependencies, and real integration-test commands.
