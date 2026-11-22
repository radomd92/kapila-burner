#!/bin/bash
set -euo pipefail
directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec "$directory/kapila-maker.app/Contents/MacOS/kapila-maker" "$@"
