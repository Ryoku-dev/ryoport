#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out="${1:-$root/dist}"
version="$(sed -n 's/^const Version = "\([0-9.]*\)"$/\1/p' "$root/internal/version/version.go")"
[[ "$version" =~ ^1\.(0|[1-9][0-9]*)\.[0-9]$ ]] || { echo "could not read a canonical version" >&2; exit 1; }

name="ryoport-$version-linux-x86_64"
staging="$(mktemp -d)"
cleanup() { rm -rf "$staging"; }
trap cleanup EXIT

make -C "$root" DESTDIR="$staging" PREFIX= install

# without the marker a fresh extract into ~/.local looks like a source build and never updates itself
marker="share/ryoport/installed"
mkdir -p "$staging/${marker%/*}"
: >"$staging/$marker"
mapfile -t files < <(cd "$staging" && find bin share \( -type f -o -type l \) | LC_ALL=C sort)
{
  printf '{\n  "version": "%s",\n  "files": [\n' "$version"
  for i in "${!files[@]}"; do
    separator=,
    (( i < ${#files[@]} - 1 )) || separator=
    printf '    "%s"%s\n' "${files[$i]}" "$separator"
  done
  printf '  ]\n}\n'
} >"$staging/$marker"
mkdir -p "$out"
out="$(cd "$out" && pwd)"
epoch="${SOURCE_DATE_EPOCH:-$(git -C "$root" log -1 --format=%ct 2>/dev/null || date +%s)}"
tar --sort=name --mtime="@$epoch" --owner=0 --group=0 --numeric-owner -C "$staging" -cf - bin share | gzip -n > "$out/$name.tar.gz"
( cd "$out" && sha256sum "$name.tar.gz" > "$name.tar.gz.sha256" )
printf 'built %s\n' "$out/$name.tar.gz"
