#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
manifest="$root/tools/icons/manifest.json"
python3 "$root/tools/icons/extract.py" --output "$manifest"
cd "$root"
go run ./tools/icons/encode "$manifest" "$root/ui/icons/icons_gen.go" "$root/ui/icons/icons.go"
