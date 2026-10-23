import std/[os, strutils]

import ./catalog

proc envOr(name, fallback: string): string =
  let value = getEnv(name)
  if value.len > 0: value else: fallback

proc dataRoot*(): string =
  let explicit = getEnv("SELEVER_DATA_DIR")
  if explicit.len > 0:
    return absolutePath(explicit)
  let home = getHomeDir()
  envOr("XDG_DATA_HOME", home / ".local" / "share") / "selever"

proc cacheRoot*(): string =
  let explicit = getEnv("SELEVER_CACHE_DIR")
  if explicit.len > 0:
    return absolutePath(explicit)
  let home = getHomeDir()
  envOr("XDG_CACHE_HOME", home / ".cache") / "selever"

proc configRoot*(): string =
  let explicit = getEnv("SELEVER_CONFIG_DIR")
  if explicit.len > 0:
    return absolutePath(explicit)
  let home = getHomeDir()
  envOr("XDG_CONFIG_HOME", home / ".config") / "selever"

proc shimDir*(): string =
  let explicit = getEnv("SELEVER_SHIMS_DIR")
  if explicit.len > 0:
    return absolutePath(explicit)
  getHomeDir() / ".local" / "bin"

proc fsComponent*(value: string): string =
  const Hex = "0123456789ABCDEF"
  for ch in value:
    if ch.isAlphaNumeric or ch in {'-', '_', '.'}:
      result.add ch
    else:
      let byte = ord(ch)
      result.add '%'
      result.add Hex[(byte shr 4) and 0xF]
      result.add Hex[byte and 0xF]

proc hexValue(ch: char): int =
  if ch in {'0' .. '9'}: ord(ch) - ord('0')
  elif ch in {'a' .. 'f'}: ord(ch) - ord('a') + 10
  elif ch in {'A' .. 'F'}: ord(ch) - ord('A') + 10
  else: -1

proc unfsComponent*(value: string): string =
  var index = 0
  while index < value.len:
    if value[index] == '%' and index + 2 < value.len:
      let high = hexValue(value[index + 1])
      let low = hexValue(value[index + 2])
      if high >= 0 and low >= 0:
        result.add char((high shl 4) or low)
        index += 3
        continue
    result.add value[index]
    inc index

proc storageName*(tool: ToolId): string =
  case tool.kind
  of tkNodejs: "node"
  of tkGolang: "go"
  of tkZig: "zig"
  of tkZls: "zls"
  of tkNpm: "npm"
  of tkGoPackage: "gopkg"
  of tkUnknown: "unknown"

proc runtimeRoot*(tool: ToolId): string =
  if not tool.isRuntime:
    raise newException(ValueError, "not a runtime: " & tool.name)
  dataRoot() / storageName(tool)

proc runtimeInstallDir*(tool: ToolId, version: string): string =
  runtimeRoot(tool) / fsComponent(version)

proc runtimeSelectorPath*(tool: ToolId, selector: string): string =
  runtimeRoot(tool) / fsComponent(selector)

proc packageRoot*(tool: ToolId): string =
  if not tool.isPackage:
    raise newException(ValueError, "not a package: " & tool.name)
  dataRoot() / storageName(tool) / fsComponent(tool.package)

proc packageParentRoot*(tool: ToolId, parentVersion: string): string =
  packageRoot(tool) / fsComponent(parentVersion)

proc packageVersionsRoot*(tool: ToolId, parentVersion: string): string =
  packageParentRoot(tool, parentVersion) / "versions"

proc packageSelectorsRoot*(tool: ToolId, parentVersion: string): string =
  packageParentRoot(tool, parentVersion) / "selectors"

proc packageInstallDir*(tool: ToolId, version, parentVersion: string): string =
  packageVersionsRoot(tool, parentVersion) / fsComponent(version)

proc packageSelectorPath*(tool: ToolId, selector,
                          parentVersion: string): string =
  packageSelectorsRoot(tool, parentVersion) / fsComponent(selector)

proc nameMapPath*(): string =
  dataRoot() / "name-map"

proc installDir*(tool: ToolId, version: string,
                 parentVersion = ""): string =
  if tool.isPackage:
    if parentVersion.len == 0:
      raise newException(ValueError,
        tool.name & " requires a " & tool.parentTool & " version")
    packageInstallDir(tool, version, parentVersion)
  else:
    runtimeInstallDir(tool, version)

proc executableDirs*(tool: ToolId, version: string,
                     parentVersion = ""): seq[string] =
  let root = installDir(tool, version, parentVersion)
  case tool.kind
  of tkNodejs:
    when defined(windows):
      result = @[root]
    else:
      result = @[root / "bin"]
  of tkGolang, tkGoPackage, tkNpm:
    result = @[root / "bin"]
  of tkZig, tkZls:
    result = @[root]
  of tkUnknown:
    discard
