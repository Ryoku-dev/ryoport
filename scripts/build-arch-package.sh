#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out="${1:-$root/dist}"

if [[ "$(id -u)" -eq 0 ]]; then
  pacman -Syu --noconfirm --needed base-devel git go pkgconf sudo
  id builder >/dev/null 2>&1 || useradd -m builder
  printf 'builder ALL=(ALL) NOPASSWD: /usr/bin/pacman\n' > /etc/sudoers.d/ryoport-builder
  chmod 0440 /etc/sudoers.d/ryoport-builder
  chown -R builder:builder "$root"
  install -d -o builder -g builder "$out"
  exec sudo -u builder -H bash "${BASH_SOURCE[0]}" "$out"
fi

command -v makepkg >/dev/null || { echo "makepkg not found" >&2; exit 1; }
version="$(sed -n 's/^const Version = "\([0-9.]*\)"$/\1/p' "$root/internal/version/version.go")"
[[ "$version" =~ ^1\.(0|[1-9][0-9]*)\.[0-9]$ ]] || { echo "could not read a canonical version" >&2; exit 1; }
name="ryoport-$version-1-x86_64.pkg.tar.zst"

cd "$root/packaging/arch"
built=""
while IFS= read -r candidate; do
  if [[ "$(basename "$candidate")" == "$name" ]]; then
    built="$candidate"
    break
  fi
done < <(makepkg --packagelist)
[[ -n "$built" ]] || { echo "makepkg will not produce $name" >&2; exit 1; }

makepkg --clean --cleanbuild --syncdeps --noconfirm --nocheck
[[ -f "$built" ]] || { echo "expected package not produced: $built" >&2; exit 1; }
mkdir -p "$out"
out="$(cd "$out" && pwd)"
cp -f "$built" "$out/$name"
( cd "$out" && sha256sum "$name" > "$name.sha256" )
printf 'built %s\n' "$out/$name"
