#!/usr/bin/env bash
# Unpacks every bundle in <dist-dir> somewhere else and checks that its
# LuaRocks installs a C rock and a pure Lua rock that its Lua then loads.
#
#   scripts/test-lua.sh <dist-dir> <platform>

set -euo pipefail

if [ $# -ne 2 ]; then
  echo "usage: $0 <dist-dir> <platform>" >&2
  exit 2
fi

dist=$1
platform=$2
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

native() {
  # Windows programs need Windows paths.
  if [ "$platform" = windows-x64 ]; then cygpath -m "$1"; else echo "$1"; fi
}

for archive in "$dist"/lua-*-"$platform".tar.gz; do
  name=$(basename "$archive" .tar.gz)
  echo "==> $name"
  tar -xzf "$archive" -C "$work"
  bundle=$work/$name
  # How selever runs LuaRocks.
  export LUAROCKS_SYSCONFDIR
  LUAROCKS_SYSCONFDIR=$(native "$bundle/etc/luarocks")
  lua_dir=--lua-dir=$(native "$bundle")
  case $platform in
    windows-x64)
      lua=$bundle/bin/lua.exe
      luarocks=("$bundle/bin/luarocks.exe" "$lua_dir")
      ;;
    macos-arm64)
      lua=$bundle/bin/lua
      luarocks=("$lua" "$bundle/bin/luarocks" "$lua_dir")
      ;;
    *)
      lua=$bundle/bin/lua
      luarocks=("$bundle/bin/luarocks" "$lua_dir")
      ;;
  esac
  "${luarocks[@]}" config platforms || true
  "${luarocks[@]}" install lpeg
  "${luarocks[@]}" install inspect
  "$lua" -e '
    local lpeg = require("lpeg")
    local inspect = require("inspect")
    print(_VERSION, lpeg.version, inspect({ 1, 2 }))'
  rm -rf "$bundle"
done
