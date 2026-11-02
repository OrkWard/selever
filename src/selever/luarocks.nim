## How selever runs LuaRocks and luabin commands of a Lua bundle built by
## scripts/build-lua.sh. The bundle's LuaRocks finds its configuration and
## Lua from where it runs, so it needs no flags or environment.

import std/os

import ./processutil

type LuaRocksCommand* = object
  executable*: string
  ## Arguments before the user's.
  arguments*: seq[string]

proc luarocksCommand*(root: string; command = "luarocks"): LuaRocksCommand =
  ## command is luarocks or luarocks-admin.
  when defined(windows):
    result.executable = root / "bin" / (command & ".exe")
  elif defined(macosx):
    # No standalone LuaRocks for macOS: the bundle runs its sources.
    result.executable = root / "bin" / "lua"
    result.arguments.add root / "bin" / command
  else:
    result.executable = root / "bin" / command

proc isInstalled*(command: LuaRocksCommand): bool =
  command.executable.isExecutableFile and
    (command.arguments.len == 0 or fileExists(command.arguments[0]))

proc luaTreePrelude*(tree, shortVersion: string): string =
  ## Lua code that puts the modules of a rocks tree ahead of the defaults.
  let extension = when defined(windows): "dll" else: "so"
  let share = tree / "share" / "lua" / shortVersion
  let lib = tree / "lib" / "lua" / shortVersion
  let path = share / "?.lua;" & share / "?" / "init.lua;"
  let cpath = lib / ("?." & extension) & ";"
  "package.path = [==[" & path & "]==] .. package.path; " &
    "package.cpath = [==[" & cpath & "]==] .. package.cpath"
