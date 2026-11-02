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

for archive in "$dist"/lua-*-"$platform".tar.gz; do
  name=$(basename "$archive" .tar.gz)
  echo "==> $name"
  tar -xzf "$archive" -C "$work"
  bundle=$work/$name
  # LuaRocks runs as is: no flags, no environment.
  case $platform in
    windows-x64)
      lua=$bundle/bin/lua.exe
      luarocks=("$bundle/bin/luarocks.exe")
      ;;
    linux-x86_64)
      lua=$bundle/bin/lua
      luarocks=("$bundle/bin/luarocks")
      ;;
    *)
      lua=$bundle/bin/lua
      luarocks=("$lua" "$bundle/bin/luarocks")
      ;;
  esac
  "${luarocks[@]}" install lpeg
  "${luarocks[@]}" install inspect
  if [ "$platform" != macos-arm64 ]; then
    # Started through PATH from elsewhere, it still finds its bundle.
    (cd / && PATH="$bundle/bin:$PATH" luarocks list | grep -q inspect)
  fi
  "$lua" -e '
    local lpeg = require("lpeg")
    local inspect = require("inspect")
    print(_VERSION, lpeg.version, inspect({ 1, 2 }))'
  rm -rf "$bundle"
done
