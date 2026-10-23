import std/[algorithm, json, os, sets, strutils]

import ./[catalog, installer, layout, lockfile, namemap]

proc runtimeCommands(installed: InstalledSelection): seq[string] =
  let root = runtimeInstallDir(installed.tool, installed.version)
  case installed.tool.kind
  of tkNodejs:
    result.add "node"
    if installed.version == "system" or installed.version.startsWith("path:"):
      result.add ["npm", "npx", "corepack"]
      return
    let moduleRoot =
      when defined(windows): root / "node_modules"
      else: root / "lib" / "node_modules"
    if fileExists(moduleRoot / "npm" / "bin" / "npm-cli.js"):
      result.add ["npm", "npx"]
    if fileExists(moduleRoot / "corepack" / "dist" / "corepack.js"):
      result.add "corepack"
  of tkGolang:
    result.add ["go", "gofmt"]
  of tkZig:
    result.add "zig"
  of tkZls:
    result.add "zls"
  else:
    discard

proc managedShimsPath(): string =
  configRoot() / "managed-shims.json"

proc loadManagedShims(): HashSet[string] =
  let path = managedShimsPath()
  if not fileExists(path):
    return
  let document = parseJson(readFile(path))
  if document.kind != JArray:
    raise newException(ValueError, "invalid managed shim registry: " & path)
  for item in document.items:
    if item.kind != JString:
      raise newException(ValueError, "invalid managed shim registry: " & path)
    result.incl item.getStr()

proc saveManagedShims(managed: HashSet[string]) =
  createDir(configRoot())
  var names: seq[string]
  for name in managed.items:
    names.add name
  names.sort()
  var document = newJArray()
  for name in names:
    document.add %name
  let path = managedShimsPath()
  let temporary = path & ".tmp"
  writeFile(temporary, $document & "\n")
  moveFile(temporary, path)

proc isLegacyManaged(command: string): bool =
  let marker = shimDir() / (command & ".shim")
  fileExists(marker) and readFile(marker).contains("# globver-managed")

proc ensureShim(command, source: string; alreadyManaged: bool) =
  if command.len == 0 or command.toLowerAscii() == "selever" or
      command.contains('/') or command.contains('\\'):
    raise newException(ValueError, "invalid shim command: " & command)

  createDir(shimDir())
  let destination =
    when defined(windows): shimDir() / (command & ".exe")
    else: shimDir() / command

  if fileExists(destination) or symlinkExists(destination):
    try:
      if sameFile(source, destination):
        return
    except OSError:
      discard
    if isLegacyManaged(command) or alreadyManaged:
      removeFile(destination)
      let marker = shimDir() / (command & ".shim")
      if fileExists(marker):
        removeFile(marker)
    else:
      raise newException(ValueError,
        "refusing to replace unmanaged shim: " & destination)

  when defined(windows):
    createHardlink(source, destination)
  else:
    createSymlink(source, destination)

proc syncShims*(installed: openArray[InstalledSelection];
                dispatcher = getAppFilename()) =
  var lock = acquireInstallLock(configRoot() / "managed-shims.lock")
  defer: lock.release()
  var managed = loadManagedShims()
  var commands = allMappedCommands()
  for selection in installed:
    if selection.tool.isRuntime:
      commands.add selection.runtimeCommands
  for command in commands:
    ensureShim(command, dispatcher, command in managed)
    managed.incl command
  saveManagedShims(managed)
