#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

tag="${1:-}"
[[ "$tag" =~ ^v1\.(0|[1-9][0-9]*)\.[0-9]$ ]] || { echo "expected a canonical v1 tag" >&2; exit 1; }
version="${tag#v}"

printf '# Ryoport %s\n\n' "$tag"
git show "$tag:CHANGELOG.md" | awk -v heading="## $tag" '
  index($0, heading) == 1 && (length($0) == length(heading) || substr($0, length(heading) + 1, 1) == " ") { active=1; next }
  active && /^## / { exit }
  active { print }
'
cat <<EOF
## Install or update on Arch or CachyOS

Download \`ryoport-$version-1-x86_64.pkg.tar.zst\` and its checksum, then run:

\`\`\`sh
sha256sum -c ryoport-$version-1-x86_64.pkg.tar.zst.sha256
sudo pacman -U ./ryoport-$version-1-x86_64.pkg.tar.zst
\`\`\`

Ryoku ships Ryoport with its v2 line, where \`ryoku update\` installs and updates it. Ryoku v1 keeps its own built-in Ryoport.

## Install on another Linux distribution

Download \`ryoport-$version-linux-x86_64.tar.gz\` and its checksum, verify it, then copy the archive contents into \`~/.local\`. Ryoport can update this installation itself.

Source and SHA-256 files are attached for Fedora, Void and Nix package builds.
EOF
