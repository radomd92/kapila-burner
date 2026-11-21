# Source this file with Bash; no system paths or packages are modified.
GTK_DEV_ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
export GTK_DEV_ROOT
export PKG_CONFIG_PATH="$GTK_DEV_ROOT/sysroot/usr/lib/x86_64-linux-gnu/pkgconfig:$GTK_DEV_ROOT/sysroot/usr/share/pkgconfig${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"
unset PKG_CONFIG_SYSROOT_DIR
export CGO_ENABLED=1
export GOCACHE="$GTK_DEV_ROOT/cache/go-build"
export GOMODCACHE="$GTK_DEV_ROOT/cache/go-mod"
export GOPATH="$GTK_DEV_ROOT/cache/go"
export GOTMPDIR="$GTK_DEV_ROOT/cache/tmp"
