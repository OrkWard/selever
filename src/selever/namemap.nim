import std/[algorithm, os, sequtils, sets, strutils]

import ./[catalog, layout, lockfile]

type NameMapEntry* = object
  command*: string
  tool*: string
  locator*: string

const ReservedCommands = [
  "node", "npm", "npx", "corepack",
  "go", "gofmt", "zig", "zls", "selever",
]

proc validCommand(command: string): bool =
  if command.len == 0 or command.contains({' ', '\t', '\r', '\n', '/', '\\'}):
    return false
  true

proc parseNameMap*(content: string): seq[NameMapEntry] =
  var seen = initHashSet[string]()
  var previous: string
  for line in content.splitLines():
    if line.len == 0:
      continue
    let fields = line.split(' ')
    if fields.len != 3 or fields.anyIt(it.len == 0):
      raise newException(ValueError, "invalid name-map line: " & line)
    if not fields[0].validCommand:
      raise newException(ValueError, "invalid command in name-map: " & fields[0])
    let tool = parseToolId(fields[1])
    if not tool.isPackage:
      raise newException(ValueError, "invalid package in name-map: " & fields[1])
    if fields[2] != "bin/" & fields[0]:
      raise newException(ValueError, "invalid locator in name-map: " & fields[2])
    if fields[0] in seen:
      raise newException(ValueError, "duplicate command in name-map: " & fields[0])
    if previous.len > 0 and cmp(previous, fields[0]) >= 0:
      raise newException(ValueError, "name-map is not sorted at: " & fields[0])
    seen.incl fields[0]
    previous = fields[0]
    result.add NameMapEntry(
      command: fields[0],
      tool: fields[1],
      locator: fields[2],
    )

proc loadNameMap*(): seq[NameMapEntry] =
  let path = nameMapPath()
  if not fileExists(path):
    return
  if getFileSize(path) > 4 * 1024 * 1024:
    raise newException(ValueError, "name-map is too large: " & path)
  parseNameMap(readFile(path))

proc saveNameMap(entries: seq[NameMapEntry]) =
  var sorted = entries
  sorted.sort(proc(a, b: NameMapEntry): int = cmp(a.command, b.command))
  var content: string
  for entry in sorted:
    content.add entry.command & " " & entry.tool & " " & entry.locator & "\n"
  createDir(dataRoot())
  let temporary = nameMapPath() & ".tmp"
  writeFile(temporary, content)
  moveFile(temporary, nameMapPath())

proc lookupName*(command: string): NameMapEntry =
  for entry in loadNameMap():
    let comparison = cmp(entry.command, command)
    if comparison == 0:
      return entry
    if comparison > 0:
      return

proc allMappedCommands*(): seq[string] =
  for entry in loadNameMap():
    result.add entry.command

proc commandsInInstall(root: string): seq[string] =
  let binDir = root / "bin"
  if not dirExists(binDir):
    return
  for kind, path in walkDir(binDir):
    if kind notin {pcFile, pcLinkToFile}:
      continue
    var name = splitFile(path).name
    when defined(windows):
      let extension = splitFile(path).ext.toLowerAscii()
      if extension notin [".exe", ".cmd", ""]:
        continue
    if name.validCommand and name notin result:
      result.add name

proc validatePackageCommands*(tool: ToolId, installRoot: string) =
  let commands = commandsInInstall(installRoot)
  if commands.len == 0:
    raise newException(ValueError, tool.name & " provides no commands")
  var lock = acquireInstallLock(dataRoot() / "name-map.lock")
  defer: lock.release()
  for command in commands:
    if command in ReservedCommands:
      raise newException(ValueError,
        command & ": command is reserved by a built-in tool")
    for entry in loadNameMap():
      if entry.command == command and entry.tool != tool.name:
        raise newException(ValueError,
          command & ": command already belongs to " & entry.tool)

proc installedPackageCommands(tool: ToolId): seq[string] =
  let root = packageRoot(tool)
  if not dirExists(root):
    return
  for parentKind, parentPath in walkDir(root):
    if parentKind != pcDir:
      continue
    let versions = parentPath / "versions"
    if not dirExists(versions):
      continue
    for versionKind, versionPath in walkDir(versions):
      if versionKind != pcDir:
        continue
      for command in commandsInInstall(versionPath):
        if command notin result:
          result.add command

proc registerPackageCommands*(tool: ToolId) =
  if not tool.isPackage:
    raise newException(ValueError, "not a package: " & tool.name)
  var lock = acquireInstallLock(dataRoot() / "name-map.lock")
  defer: lock.release()

  let commands = installedPackageCommands(tool)
  let existing = loadNameMap()
  var updated: seq[NameMapEntry]
  for entry in existing:
    if entry.tool == tool.name:
      continue
    if entry.command in commands:
      raise newException(ValueError,
        entry.command & ": command already belongs to " & entry.tool)
    updated.add entry

  for command in commands:
    if command in ReservedCommands:
      raise newException(ValueError,
        command & ": command is reserved by a built-in tool")
    updated.add NameMapEntry(
      command: command,
      tool: tool.name,
      locator: "bin/" & command,
    )
  saveNameMap(updated)
