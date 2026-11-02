## How selever runs LuaRocks and luabin commands of a Lua bundle built by
## scripts/build-lua.sh.

import std/os

import ./processutil

type LuaRocksCommand* = object
  executable*: string
  ## Arguments before the user's.
  arguments*: seq[string]
  ## Environment variables to set.
  variables*: seq[(string, string)]

proc luarocksCommand*(root: string; command = "luarocks"): LuaRocksCommand =
  ## command is luarocks or luarocks-admin. The configuration in
  ## <root>/etc/luarocks installs rocks into the bundle, and --lua-dir
  ## keeps LuaRocks from probing every Lua on PATH.
  when defined(windows):
    result.executable = root / "bin" / (command & ".exe")
  else:
    # Outside Windows the bundle runs LuaRocks' sources with its lua.
    result.executable = root / "bin" / "lua"
    result.arguments.add root / "bin" / command
  result.arguments.add "--lua-dir=" & root
  result.variables.add ("LUAROCKS_SYSCONFDIR", root / "etc" / "luarocks")

proc isInstalled*(command: LuaRocksCommand): bool =
  when defined(windows):
    command.executable.isExecutableFile
  else:
    command.executable.isExecutableFile and fileExists(command.arguments[0])

proc luaTreePrelude*(tree, shortVersion: string): string =
  ## Lua code that puts the modules of a rocks tree ahead of the defaults.
  let extension = when defined(windows): "dll" else: "so"
  let share = tree / "share" / "lua" / shortVersion
  let lib = tree / "lib" / "lua" / shortVersion
  let path = share / "?.lua;" & share / "?" / "init.lua;"
  let cpath = lib / ("?." & extension) & ";"
  "package.path = [==[" & path & "]==] .. package.path; " &
    "package.cpath = [==[" & cpath & "]==] .. package.cpath"
