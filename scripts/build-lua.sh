#!/usr/bin/env bash
# Builds a relocatable Lua + LuaRocks bundle for selever.
#
#   scripts/build-lua.sh <lua-version> <platform> <luarocks-version> <output-dir>
#
# Platforms: windows-x64 (MSYS2 UCRT64 gcc), linux-x86_64, macos-arm64.
# Writes <output-dir>/lua-<version>-<platform>.tar.gz and its .sha256.
#
# Bundle layout, under lua-<version>-<platform>/:
#   bin/                 lua, luac, luarocks, luarocks-admin (and lua5X.dll)
#   include/             lua.h luaconf.h lualib.h lauxlib.h lua.hpp
#   lib/                 liblua.a (not on Windows)
#   share/lua/<V>/       Lua modules; also LuaRocks itself on macOS
#   lib/lua/<V>/         C modules
#   etc/luarocks/        LuaRocks configuration
#
# Lua resolves "!" in its default package paths to the directory of the
# running executable, so modules installed by LuaRocks into the bundle are
# found without LUA_PATH or LUA_INIT. LuaRocks reads its configuration from
# LUAROCKS_SYSCONFDIR, which selever sets to <bundle>/etc/luarocks.

set -euo pipefail

if [ $# -ne 4 ]; then
  echo "usage: $0 <lua-version> <platform> <luarocks-version> <output-dir>" >&2
  exit 2
fi

version=$1
platform=$2
luarocks_version=$3
output=$4

here=$(cd "$(dirname "$0")" && pwd)
short=$(echo "$version" | cut -d. -f1-2)
nodot=$(echo "$short" | tr -d .)
name=lua-$version-$platform

die() {
  echo "build-lua: $*" >&2
  exit 1
}

case $platform in
  windows-x64 | linux-x86_64 | macos-arm64) ;;
  *) die "unsupported platform: $platform" ;;
esac
case $short in
  5.1 | 5.4 | 5.5) ;;
  *) die "unsupported Lua version: $version" ;;
esac

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
root=$work/$name

fetch() {
  curl -fsSL --retry 3 -o "$2" "$1"
}

sha256() {
  if command -v sha256sum > /dev/null; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

# ------------------------------------------------------------------ source

echo "==> Lua $version source"
archive=lua-$version.tar.gz
fetch "https://www.lua.org/ftp/$archive" "$work/$archive"
# lua.org/ftp lists every tarball with its SHA-256 in a following cell.
expected=$(curl -fsSL --retry 3 https://www.lua.org/ftp/ | awk -v file="$archive" '
  index($0, "HREF=\"" file "\"") { found = 1 }
  found && match($0, /CLASS="sum">[0-9a-f]+</) {
    print substr($0, RSTART + 12, RLENGTH - 13)
    exit
  }')
[ -n "$expected" ] || die "no SHA-256 for $archive on lua.org"
actual=$(sha256 "$work/$archive")
[ "$actual" = "$expected" ] || die "SHA-256 mismatch for $archive: $actual != $expected"
tar -xzf "$work/$archive" -C "$work"
src=$work/lua-$version/src

# ------------------------------------------------------------------ patches

pattern='^#define setprogdir(L)[[:space:]]*((void)0)$'
[ "$(grep -c "$pattern" "$src/loadlib.c")" = 1 ] ||
  die "loadlib.c: expected one no-op setprogdir"
cp "$here/lua/selever_progdir.h" "$src/"
sed -i.orig "s/$pattern/#include \"selever_progdir.h\"/" "$src/loadlib.c"
grep -q '#include "selever_progdir.h"' "$src/loadlib.c" ||
  die "loadlib.c: patch did not apply"

if [ "$platform" = windows-x64 ]; then
  sep='\\'
  ext=dll
else
  sep=/
  ext=so
fi
share="!$sep..${sep}share${sep}lua$sep$short$sep"
clib="!$sep..${sep}lib${sep}lua$sep$short$sep"
here_dir=".$sep"
cat > "$work/paths.h" << EOF
/* selever: modules live in the bundle that holds the executable */
#undef LUA_PATH_DEFAULT
#undef LUA_CPATH_DEFAULT
#define LUA_PATH_DEFAULT \\
  "${share}?.lua;${share}?${sep}init.lua;" \\
  "${clib}?.lua;${clib}?${sep}init.lua;" \\
  "${here_dir}?.lua;${here_dir}?${sep}init.lua"
#define LUA_CPATH_DEFAULT \\
  "${clib}?.$ext;${clib}loadall.$ext;${here_dir}?.$ext"

EOF
# Insert before the include guard's closing #endif, the last one in the file.
last=$(grep -n '^#endif' "$src/luaconf.h" | tail -n 1 | cut -d: -f1)
[ -n "$last" ] || die "luaconf.h: no closing #endif"
awk -v at="$last" -v block="$work/paths.h" '
  NR == at { while ((getline line < block) > 0) print line }
  { print }' "$src/luaconf.h" > "$work/luaconf.h"
mv "$work/luaconf.h" "$src/luaconf.h"

# ------------------------------------------------------------------ compile

cc=${CC:-gcc}
case $short in
  5.1) cflags="-O2 -Wall" ;;
  5.4) cflags="-std=gnu99 -O2 -Wall -Wextra -DLUA_COMPAT_5_3" ;;
  *) cflags="-std=gnu99 -O2 -Wall -Wextra" ;;
esac
libs=""
case $platform in
  windows-x64)
    sysflags="-DLUA_BUILD_AS_DLL"
    ;;
  linux-x86_64)
    # No readline: the bundle must not need libreadline at load time.
    case $short in
      5.1) sysflags="-DLUA_USE_POSIX -DLUA_USE_DLOPEN" ;;
      5.4) sysflags="-DLUA_USE_LINUX" ;;
      # 5.5 opens readline with dlopen when it exists.
      *) sysflags="-DLUA_USE_LINUX -DLUA_READLINELIB=\"libreadline.so.8\"" ;;
    esac
    libs="-Wl,-E -ldl -lm"
    ;;
  macos-arm64)
    # macOS ships libreadline as libedit.
    case $short in
      5.1) sysflags="-DLUA_USE_LINUX" ;;
      5.4) sysflags="-DLUA_USE_MACOSX -DLUA_USE_READLINE" ;;
      *) sysflags="-DLUA_USE_MACOSX" ;;
    esac
    libs="-lreadline"
    ;;
esac

echo "==> compile ($cc $cflags $sysflags)"
objdir=$work/obj
mkdir -p "$objdir" "$root/bin" "$root/include" "$root/lib" \
  "$root/share/lua/$short" "$root/lib/lua/$short" "$root/etc/luarocks"
core=()
for file in "$src"/*.c; do
  base=$(basename "$file" .c)
  # shellcheck disable=SC2086
  $cc $cflags $sysflags -c -o "$objdir/$base.o" "$file"
  case $base in
    lua | luac | print) ;;
    *) core+=("$objdir/$base.o") ;;
  esac
done
luac_objs=("$objdir/luac.o")
[ -f "$objdir/print.o" ] && luac_objs+=("$objdir/print.o")

ar rcs "$work/liblua.a" "${core[@]}"
if [ "$platform" = windows-x64 ]; then
  dll=lua$nodot.dll
  $cc -shared -s -static-libgcc -o "$root/bin/$dll" "${core[@]}"
  $cc -s -static-libgcc -o "$root/bin/lua.exe" "$objdir/lua.o" "$root/bin/$dll"
  $cc -s -static-libgcc -o "$root/bin/luac.exe" "${luac_objs[@]}" "$work/liblua.a"
  lua=$root/bin/lua.exe
else
  cp "$work/liblua.a" "$root/lib/liblua.a"
  # shellcheck disable=SC2086
  $cc -o "$root/bin/lua" "$objdir/lua.o" "$root/lib/liblua.a" $libs
  # shellcheck disable=SC2086
  $cc -o "$root/bin/luac" "${luac_objs[@]}" "$root/lib/liblua.a" $libs
  lua=$root/bin/lua
fi

headers="lua.h luaconf.h lualib.h lauxlib.h"
for header in $headers; do
  cp "$src/$header" "$root/include/"
done
if [ -f "$src/lua.hpp" ]; then
  cp "$src/lua.hpp" "$root/include/"
else
  cp "$work/lua-$version/etc/lua.hpp" "$root/include/"
fi
cp "$work/lua-$version/doc/readme.html" "$root/lua-readme.html" 2> /dev/null || true

# ------------------------------------------------------------------ luarocks

echo "==> LuaRocks $luarocks_version"
releases=https://luarocks.github.io/luarocks/releases
case $platform in
  windows-x64 | linux-x86_64)
    if [ "$platform" = windows-x64 ]; then
      zip=luarocks-$luarocks_version-windows-64.zip
      suffix=.exe
    else
      zip=luarocks-$luarocks_version-linux-x86_64.zip
      suffix=
    fi
    fetch "$releases/$zip" "$work/$zip"
    if command -v unzip > /dev/null; then
      unzip -q "$work/$zip" -d "$work/luarocks"
    else
      python3 -m zipfile -e "$work/$zip" "$work/luarocks"
    fi
    for tool in luarocks luarocks-admin; do
      found=$(find "$work/luarocks" -type f -name "$tool$suffix" | head -n 1)
      [ -n "$found" ] || die "$zip has no $tool$suffix"
      cp "$found" "$root/bin/$tool$suffix"
      chmod +x "$root/bin/$tool$suffix"
    done
    ;;
  macos-arm64)
    # No standalone binary for macOS: run LuaRocks' sources with this Lua.
    tarball=luarocks-$luarocks_version.tar.gz
    fetch "$releases/$tarball" "$work/$tarball"
    tar -xzf "$work/$tarball" -C "$work"
    lrsrc=$work/luarocks-$luarocks_version/src
    cp -R "$lrsrc/luarocks" "$root/share/lua/$short/"
    if [ "$short" = 5.1 ] && [ -d "$lrsrc/compat53" ]; then
      cp -R "$lrsrc/compat53" "$root/share/lua/$short/"
    fi
    for tool in luarocks luarocks-admin; do
      {
        echo "-- selever: run with the bundle's lua; configuration is in"
        echo "-- <bundle>/etc/luarocks unless LUAROCKS_SYSCONFDIR says otherwise."
        echo 'local bundle = arg[0]:gsub("[/\\][^/\\]*$", ""):gsub("[/\\][^/\\]*$", "")'
        echo 'package.loaded["luarocks.core.hardcoded"] = { SYSCONFDIR = bundle .. "/etc/luarocks" }'
        tail -n +2 "$lrsrc/bin/$tool"
      } > "$root/bin/$tool"
    done
    ;;
esac

if [ "$platform" = windows-x64 ]; then
  lualib="   LUALIB = \"$dll\","
  libdir=bin
  interpreter=lua.exe
else
  lualib=""
  libdir=lib
  interpreter=lua
fi
cat > "$root/etc/luarocks/config-$short.lua" << EOF
-- LuaRocks configuration of a selever Lua bundle. selever sets
-- LUAROCKS_SYSCONFDIR to <bundle>/etc/luarocks; everything else follows.
local bundle = os_getenv("LUAROCKS_SYSCONFDIR")
   :gsub("[/\\\\]+$", ""):gsub("[/\\\\][^/\\\\]+[/\\\\][^/\\\\]+$", "")

rocks_trees = {
   { name = "bundle", root = bundle },
}
variables = {
   LUA_DIR = bundle,
   LUA_BINDIR = bundle .. "/bin",
   LUA_INCDIR = bundle .. "/include",
   LUA_LIBDIR = bundle .. "/$libdir",
   LUA = bundle .. "/bin/$interpreter",
$lualib
}
EOF
# The standalone LuaRocks binaries default to the Lua they embed (5.4);
# this makes them pick the bundle's version and its config file.
echo "return \"$short\"" > "$root/etc/luarocks/default-lua-version.lua"

# ------------------------------------------------------------------ check

echo "==> check"
"$lua" -e "
  assert(_VERSION == 'Lua $short', _VERSION)
  assert(not package.path:find('!', 1, true), package.path)
  assert(package.path:find('share', 1, true), package.path)
  print(package.path)
  print(package.cpath)"

# ------------------------------------------------------------------ package

mkdir -p "$output"
tar -czf "$output/$name.tar.gz" -C "$work" "$name"
sum=$(sha256 "$output/$name.tar.gz")
echo "$sum  $name.tar.gz" > "$output/$name.tar.gz.sha256"
echo "==> $output/$name.tar.gz $sum"
