#!/bin/bash
set -euo pipefail
directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec "$directory/Kapila Burner.app/Contents/MacOS/kapila-burner" "$@"
