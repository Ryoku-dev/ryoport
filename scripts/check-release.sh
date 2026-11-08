#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

version="$(sed -n 's/^const Version = "\([0-9.]*\)"$/\1/p' internal/version/version.go)"
changelog="$(sed -n 's/^## v\([0-9][0-9.]*\).*$/\1/p' CHANGELOG.md | sed -n '1p')"
package="$(sed -n 's/^pkgver=//p' packaging/arch/PKGBUILD | sed -n '1p')"
release="$(sed -n 's/.*<release version="\([0-9.]*\)".*/\1/p' packaging/linux/dev.ryoku.ryoport.metainfo.xml | sed -n '1p')"
flake="$(sed -n 's/^        version = "\([0-9.]*\)";$/\1/p' flake.nix | sed -n '1p')"

[[ "$version" =~ ^1\.(0|[1-9][0-9]*)\.[0-9]$ ]] || { echo "invalid version: $version" >&2; exit 1; }
[[ "$version" == "$changelog" ]] || { echo "version.go and CHANGELOG.md disagree" >&2; exit 1; }
[[ "$version" == "$package" ]] || { echo "version.go and PKGBUILD disagree" >&2; exit 1; }
[[ "$version" == "$release" ]] || { echo "version.go and AppStream metadata disagree" >&2; exit 1; }
[[ "$version" == "$flake" ]] || { echo "version.go and flake.nix disagree" >&2; exit 1; }
grep -qx 'pkgrel=1' packaging/arch/PKGBUILD || { echo "PKGBUILD pkgrel must be 1" >&2; exit 1; }
cmp -s CHANGELOG.md internal/version/CHANGELOG.md || { echo "embedded changelog is stale" >&2; exit 1; }
printf 'release invariants: v%s\n' "$version"
