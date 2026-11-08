#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
out="$root/ui/theme/fonts"
subset="${PYFTSUBSET:-$HOME/.local/bin/pyftsubset}"
unicodes='U+0020-007E,U+00A0-00FF,U+2010-2027,U+2030-205E,U+2190-21FF,U+2212,U+2500-257F,U+25A0-25FF,U+2026'

mkdir -p "$out"
install -m 0644 /usr/share/fonts/OTF/SpaceGrotesk-Regular.otf "$out/SpaceGrotesk-Regular.otf"
install -m 0644 /usr/share/fonts/OTF/SpaceGrotesk-Medium.otf "$out/SpaceGrotesk-Medium.otf"
install -m 0644 /usr/share/fonts/OTF/SpaceGrotesk-Bold.otf "$out/SpaceGrotesk-Bold.otf"
license_tmp="$(mktemp)"
trap 'rm -f "$license_tmp"' EXIT
printf '%s\n\n' 'Copyright 2020 The Space Grotesk Project Authors (https://github.com/floriankarsten/space-grotesk)' > "$license_tmp"
cat /usr/share/licenses/ttf-space-mono-nerd/OFL.txt >> "$license_tmp"
install -m 0644 "$license_tmp" "$out/OFL.txt"

for weight in Regular Bold; do
    "$subset" "/usr/share/fonts/TTF/SpaceMonoNerdFont-${weight}.ttf" \
        --output-file="$out/SpaceMono-${weight}.ttf" \
        --unicodes="$unicodes" \
        --layout-features='*' \
        --glyph-names \
        --symbol-cmap \
        --legacy-cmap \
        --name-IDs='*' \
        --name-legacy \
        --name-languages='*'
    python3 "$root/tools/fonts/rename.py" "$out/SpaceMono-${weight}.ttf" "$weight"
done
