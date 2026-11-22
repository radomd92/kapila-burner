#!/usr/bin/env bash
set -euo pipefail

# Build-time requirements: Intel-capable Homebrew GTK 3, Python 3 and Xcode tools.
# Utility executables must already be installed in STAGE/tools/bin.
scripts=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec python3 "$scripts/macruntime/bundle.py" "$@"
