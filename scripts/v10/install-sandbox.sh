#!/bin/sh
# Build only into a caller-selected sandbox prefix. No host config or live install.
set -eu
if [ "$#" -ne 2 ] || [ "$1" != "--prefix" ]; then
  printf '%s\n' 'usage: scripts/v10/install-sandbox.sh --prefix /absolute/sandbox-prefix' >&2
  exit 2
fi
haft_prefix=$2
case "$haft_prefix" in
  /*) ;;
  *) printf '%s\n' 'sandbox prefix must be absolute' >&2; exit 2 ;;
esac
case "$haft_prefix" in
  */../*|*/./*|*/..|*/.) printf '%s\n' 'sandbox prefix must be normalized' >&2; exit 2 ;;
esac
# Resolve the nearest existing ancestor before creating directories, so a parent
# symlink cannot cause even an intermediate write below a live installation.
haft_ancestor=$haft_prefix
haft_suffix=
while [ ! -d "$haft_ancestor" ]; do
  if [ -e "$haft_ancestor" ] || [ -L "$haft_ancestor" ]; then
    printf '%s\n' 'sandbox prefix ancestor is not a directory' >&2; exit 2
  fi
  haft_suffix="/$(basename -- "$haft_ancestor")$haft_suffix"
  haft_ancestor=$(dirname -- "$haft_ancestor")
done
haft_ancestor=$(CDPATH= cd -- "$haft_ancestor" && pwd -P)
haft_prefix="$haft_ancestor$haft_suffix"
haft_live_home=$(CDPATH= cd -- "$HOME" && pwd -P)
case "$haft_prefix" in
  /|/usr|/usr/*|/opt|/opt/homebrew|/opt/homebrew/*|/bin|/bin/*|/sbin|/sbin/*|"$haft_live_home"|"$haft_live_home/.local"|"$haft_live_home/.local/"*|"$haft_live_home/.haft"|"$haft_live_home/.haft/"*|"$haft_live_home/.codex"|"$haft_live_home/.codex/"*)
    printf '%s\n' 'refusing a live or system installation prefix' >&2; exit 2 ;;
esac
haft_repo=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd -P)
haft_sha=$(git -C "$haft_repo" rev-parse HEAD)
if [ -L "$haft_prefix/bin" ] || [ -L "$haft_prefix/bin/haft10" ]; then
  printf '%s\n' 'sandbox destination must not be a symlink' >&2; exit 2
fi
if [ -e "$haft_prefix/bin/haft10" ] && [ ! -f "$haft_prefix/bin/haft10" ]; then
  printf '%s\n' 'sandbox binary destination must be a regular file' >&2; exit 2
fi
mkdir -p -- "$haft_prefix/bin"
haft_temp=$(mktemp "$haft_prefix/bin/.haft10-build.XXXXXX")
trap 'rm -f -- "$haft_temp"' EXIT HUP INT TERM
cd -- "$haft_repo"
go build -trimpath -ldflags "-X main.Version=$haft_sha" -o "$haft_temp" ./cmd/haft10
chmod 0755 "$haft_temp"
mv -- "$haft_temp" "$haft_prefix/bin/haft10"
printf '%s\n' "$haft_prefix/bin/haft10"
