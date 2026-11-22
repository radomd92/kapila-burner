#!/usr/bin/env bash
set -euo pipefail

# Requires Python 3; Windows also needs pacman, pacman-conf, cygpath and tar
# (with zstd support). macOS needs the same Homebrew used by the bundler.
stage=${1:?Usage: collect-runtime-sources.sh STAGE}
[[ $# == 1 ]] || { printf 'Usage: collect-runtime-sources.sh STAGE\n' >&2; exit 1; }
python=${PYTHON_NATIVE:-python3}
if command -v cygpath >/dev/null && [[ $python == *:* ]]; then python=$(cygpath -u "$python"); fi
command -v "$python" >/dev/null || { printf 'Runtime source collection requires Python 3.\n' >&2; exit 1; }
exec "$python" "$(dirname "$0")/collect-runtime-sources.py" "$stage"
