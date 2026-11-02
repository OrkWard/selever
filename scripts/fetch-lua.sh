#!/usr/bin/env bash
# Downloads Lua source tarballs, verified against the SHA-256 sums lua.org
# lists, so that builds on runners that cannot reach lua.org can use them.
#
#   scripts/fetch-lua.sh <output-dir> <lua-version>...
#
# Writes lua-<version>.tar.gz and lua-<version>.tar.gz.sha256 per version;
# build-lua.sh uses them when LUA_SOURCES names <output-dir>.

set -euo pipefail

if [ $# -lt 2 ]; then
  echo "usage: $0 <output-dir> <lua-version>..." >&2
  exit 2
fi

output=$1
shift
mkdir -p "$output"

curl_retry=(--retry 5 --retry-all-errors --retry-delay 3)
listing=$(curl -fsSL "${curl_retry[@]}" https://www.lua.org/ftp/)

sha256() {
  if command -v sha256sum > /dev/null; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

for version in "$@"; do
  archive=lua-$version.tar.gz
  # lua.org/ftp lists every tarball with its SHA-256 in a following cell.
  expected=$(echo "$listing" | awk -v file="$archive" '
    index($0, "HREF=\"" file "\"") { found = 1 }
    found && match($0, /CLASS="sum">[0-9a-f]+</) {
      print substr($0, RSTART + 12, RLENGTH - 13)
      exit
    }')
  if [ -z "$expected" ]; then
    echo "fetch-lua: no SHA-256 for $archive on lua.org" >&2
    exit 1
  fi
  curl -fsSL "${curl_retry[@]}" -o "$output/$archive" \
    "https://www.lua.org/ftp/$archive"
  actual=$(sha256 "$output/$archive")
  if [ "$actual" != "$expected" ]; then
    echo "fetch-lua: SHA-256 mismatch for $archive: $actual != $expected" >&2
    exit 1
  fi
  echo "$expected" > "$output/$archive.sha256"
  echo "==> $archive $expected"
done
