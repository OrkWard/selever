#!/usr/bin/env bash
# Builds standalone LuaRocks binaries for selever's Lua bundles.
#
#   scripts/build-luarocks.sh <luarocks-version> <output-dir>
#
# Runs on Linux x86_64 with mingw-w64, like LuaRocks' own releases, and
# writes <output-dir>/linux-x86_64/{luarocks,luarocks-admin} and
# <output-dir>/windows-x64/{luarocks,luarocks-admin}.exe.
#
# The stock binaries carry fixed configuration paths. These carry
# scripts/lua/hardcoded.lua instead, which takes the paths from the bundle
# the binary runs in, so they need no flags or environment variables.

set -euo pipefail

if [ $# -ne 2 ]; then
  echo "usage: $0 <luarocks-version> <output-dir>" >&2
  exit 2
fi

version=$1
output=$(mkdir -p "$2" && cd "$2" && pwd)
here=$(cd "$(dirname "$0")" && pwd)
host_lua=5.4.8

die() {
  echo "build-luarocks: $*" >&2
  exit 1
}

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cd "$work"

fetch() {
  curl -fsSL --retry 5 --retry-all-errors --retry-delay 3 -o "$2" "$1"
}

echo "==> host Lua $host_lua"
# Runs the build and is linked into the Linux binary.
fetch "https://www.lua.org/ftp/lua-$host_lua.tar.gz" lua.tar.gz
tar -xzf lua.tar.gz
make -s -C "lua-$host_lua" linux > /dev/null
make -s -C "lua-$host_lua" local > /dev/null
lua_dir=$work/lua-$host_lua/install

echo "==> LuaRocks $version source"
fetch "https://github.com/luarocks/luarocks/archive/refs/tags/v$version.tar.gz" \
  luarocks.tar.gz
tar -xzf luarocks.tar.gz
cd "luarocks-$version"

all_in_one=binary/all_in_one
grep -q '^local function write_hardcoded_module(dir)$' "$all_in_one" ||
  die "$all_in_one: no write_hardcoded_module"
awk -v replacement="$here/lua/write_hardcoded_module.lua" '
  /^local function write_hardcoded_module\(dir\)$/ {
    while ((getline line < replacement) > 0) print line
    skip = 1
    next
  }
  skip && /^end$/ { skip = 0; next }
  !skip { print }' "$all_in_one" > "$work/all_in_one"
mv "$work/all_in_one" "$all_in_one"
grep -q SELEVER_HARDCODED "$all_in_one" || die "$all_in_one: patch did not apply"
export SELEVER_HARDCODED=$here/lua/hardcoded.lua

./configure --lua-version=5.4 --with-lua="$lua_dir" > /dev/null

echo "==> linux-x86_64"
make binary
mkdir -p "$output/linux-x86_64"
cp build-binary/luarocks.exe "$output/linux-x86_64/luarocks"
cp build-binary/luarocks-admin.exe "$output/linux-x86_64/luarocks-admin"

echo "==> windows-x64"
# Fetch the dependency sources binary/Makefile.windows would fetch, with
# retries and from where old versions stay.
setting() {
  sed -n "s/^$1=//p" binary/Makefile.windows
}
deps=build-windows-deps-x86_64-w64-mingw32
mkdir -p "$deps"
lua_version=$(setting LIBLUA_VERSION)
openssl_version=$(setting OPENSSL_VERSION)
zlib_version=$(setting ZLIB_VERSION)
bzip2_version=$(setting BZIP2_VERSION)
fetch "https://www.lua.org/ftp/lua-$lua_version.tar.gz" \
  "$deps/lua-$lua_version.tar.gz"
fetch "https://github.com/openssl/openssl/releases/download/OpenSSL_$(echo "$openssl_version" | tr . _)/openssl-$openssl_version.tar.gz" \
  "$deps/openssl-$openssl_version.tar.gz"
fetch "https://zlib.net/fossils/zlib-$zlib_version.tar.gz" \
  "$deps/zlib-$zlib_version.tar.gz"
fetch "https://sourceware.org/pub/bzip2/bzip2-$bzip2_version.tar.gz" \
  "$deps/bzip2-$bzip2_version.tar.gz"
make windows-binary-64 MINGW_SYSROOT=/usr/x86_64-w64-mingw32
mkdir -p "$output/windows-x64"
cp build-windows-binary-x86_64-w64-mingw32/luarocks.exe "$output/windows-x64/"
cp build-windows-binary-x86_64-w64-mingw32/luarocks-admin.exe "$output/windows-x64/"

ls -l "$output"/*
