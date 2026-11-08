#!/usr/bin/env bash
set -euo pipefail

if [[ -n "${RYOPORT_TAGS+x}" ]]; then
  tags="$RYOPORT_TAGS"
else
  tags="$(git tag -l 'v1.*')"
fi

best_minor=-1
best_patch=-1
for tag in $tags; do
  if [[ "$tag" =~ ^v1\.(0|[1-9][0-9]*)\.([0-9])$ ]]; then
    minor="${BASH_REMATCH[1]}"
    patch="${BASH_REMATCH[2]}"
    if (( minor > best_minor || (minor == best_minor && patch > best_patch) )); then
      best_minor="$minor"
      best_patch="$patch"
    fi
  fi
done

if (( best_minor < 0 )); then
  printf '1.0.0\n'
  exit 0
fi

minor="$best_minor"
patch=$((best_patch + 1))
if (( patch > 9 )); then
  patch=0
  minor=$((minor + 1))
fi
printf '1.%d.%d\n' "$minor" "$patch"
