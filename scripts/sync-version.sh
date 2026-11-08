#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

version="${1:-}"
mode="${2:-}"
[[ "$version" =~ ^1\.(0|[1-9][0-9]*)\.[0-9]$ ]] || { echo "expected a canonical v1 version" >&2; exit 1; }
[[ -z "$mode" || "$mode" == "--release" ]] || { echo "usage: $0 VERSION [--release]" >&2; exit 1; }

top="$(sed -n 's/^## v\([0-9][0-9.]*\).*$/\1/p' CHANGELOG.md | sed -n '1p')"
changes_file=""
cleanup() {
  [[ -z "$changes_file" ]] || rm -f "$changes_file"
}
trap cleanup EXIT

if [[ "$mode" == "--release" && "$top" != "$version" ]]; then
  previous=""
  while IFS= read -r tag; do
    if [[ "$tag" =~ ^v1\.(0|[1-9][0-9]*)\.[0-9]$ && "$tag" != "v$version" ]]; then
      previous="$tag"
      break
    fi
  done < <(git tag -l 'v1.*' --sort=-v:refname)

  range="HEAD"
  [[ -z "$previous" ]] || range="$previous..HEAD"
  changes_file="$(mktemp)"
  while IFS= read -r subject; do
    [[ -z "$subject" || "$subject" == release:* ]] && continue
    printf -- '- %s\n' "$subject" >> "$changes_file"
  done < <(git log --no-merges --format='%s' "$range")
  [[ -s "$changes_file" ]] || { echo "no non-release changes for v$version" >&2; exit 1; }

  tmp="$(mktemp .CHANGELOG.md.XXXXXX)"
  {
    printf '# Changelog\n\n## v%s\n\n' "$version"
    cat "$changes_file"
    printf '\n'
    sed -n '3,$p' CHANGELOG.md
  } > "$tmp"
  chmod 0644 "$tmp"
  mv "$tmp" CHANGELOG.md
elif [[ "$top" != "$version" ]]; then
  echo "CHANGELOG.md starts at v$top; use --release to add v$version" >&2
  exit 1
fi

version_tmp="$(mktemp internal/version/.version.go.XXXXXX)"
awk -v version="$version" '{ if ($0 ~ /^const Version = "/) print "const Version = \"" version "\""; else print }' internal/version/version.go > "$version_tmp"
chmod 0644 "$version_tmp"
mv "$version_tmp" internal/version/version.go

pkg_tmp="$(mktemp packaging/arch/.PKGBUILD.XXXXXX)"
awk -v version="$version" '{ if ($0 ~ /^pkgver=/) print "pkgver=" version; else if ($0 ~ /^pkgrel=/) print "pkgrel=1"; else print }' packaging/arch/PKGBUILD > "$pkg_tmp"
chmod 0644 "$pkg_tmp"
mv "$pkg_tmp" packaging/arch/PKGBUILD

flake_tmp="$(mktemp .flake.nix.XXXXXX)"
awk -v version="$version" '{ if ($0 ~ /^        version = "/) print "        version = \"" version "\";"; else print }' flake.nix > "$flake_tmp"
chmod 0644 "$flake_tmp"
mv "$flake_tmp" flake.nix

if ! grep -Fq "<release version=\"$version\"" packaging/linux/dev.ryoku.ryoport.metainfo.xml; then
  meta_tmp="$(mktemp packaging/linux/.metainfo.xml.XXXXXX)"
  awk -v version="$version" -v date="$(date -u +%F)" '{ print; if ($0 ~ /<releases>/) print "    <release version=\"" version "\" date=\"" date "\"/>" }' packaging/linux/dev.ryoku.ryoport.metainfo.xml > "$meta_tmp"
  chmod 0644 "$meta_tmp"
  mv "$meta_tmp" packaging/linux/dev.ryoku.ryoport.metainfo.xml
fi

embedded_tmp="$(mktemp internal/version/.CHANGELOG.md.XXXXXX)"
cp CHANGELOG.md "$embedded_tmp"
chmod 0644 "$embedded_tmp"
mv "$embedded_tmp" internal/version/CHANGELOG.md
printf 'sync-version: %s\n' "$version"
