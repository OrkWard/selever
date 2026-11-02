import std/[algorithm, os, sequtils, sets, strutils]

import ./[catalog, layout, lockfile, processutil]

type
  NameMapEntry* = object
    command*: string
    tool*: string
    locator*: string

  NameMapTransaction* = object
    lock: InstallLock
    original: seq[NameMapEntry]
    active: bool
    committed: bool

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
    if kind notin {pcFile, pcLinkToFile} or not path.isExecutableFile:
      continue
    let name =
      when defined(windows): splitFile(path).name
      else: splitPath(path).tail
    when defined(windows):
      let extension = splitFile(path).ext.toLowerAscii()
      if extension notin [".exe", ".cmd", ""]:
        continue
    if name.validCommand and name notin result:
      result.add name

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

proc beginNameMapTransaction*(): NameMapTransaction =
  result.lock = acquireInstallLock(dataRoot() / "name-map.lock")
  try:
    result.original = loadNameMap()
    result.active = true
  except CatchableError:
    result.lock.release()
    raise

proc finishNameMapTransaction*(transaction: var NameMapTransaction) =
  if not transaction.active:
    return
  try:
    if not transaction.committed:
      saveNameMap(transaction.original)
  finally:
    transaction.lock.release()
    transaction.active = false

proc commitNameMapTransaction*(transaction: var NameMapTransaction) =
  if not transaction.active:
    raise newException(ValueError, "name-map transaction is not active")
  transaction.committed = true

proc validateCommands(tool: ToolId, commands: openArray[string],
                      existing: openArray[NameMapEntry]) =
  if commands.len == 0:
    raise newException(ValueError, tool.name & " provides no commands")
  for command in commands:
    if command in ReservedCommands:
      raise newException(ValueError,
        command & ": command is reserved by a built-in tool")
    for entry in existing:
      if entry.command == command and entry.tool != tool.name:
        raise newException(ValueError,
          command & ": command already belongs to " & entry.tool)

proc reservePackageCommands*(transaction: var NameMapTransaction,
                             tool: ToolId, installRoot: string) =
  if not transaction.active:
    raise newException(ValueError, "name-map transaction is not active")
  let commands = commandsInInstall(installRoot)
  let existing = loadNameMap()
  validateCommands(tool, commands, existing)
  var updated = existing
  for command in commands:
    var found = false
    for entry in existing:
      if entry.command == command:
        found = true
        break
    if not found:
      updated.add NameMapEntry(
        command: command,
        tool: tool.name,
        locator: "bin/" & command,
      )
  saveNameMap(updated)

proc registerPackageCommands*(transaction: var NameMapTransaction,
                              tool: ToolId) =
  if not transaction.active:
    raise newException(ValueError, "name-map transaction is not active")
  if not tool.isPackage:
    raise newException(ValueError, "not a package: " & tool.name)

  let commands = installedPackageCommands(tool)
  let existing = loadNameMap()
  validateCommands(tool, commands, existing)
  var updated: seq[NameMapEntry]
  for entry in existing:
    if entry.tool == tool.name:
      continue
    updated.add entry

  for command in commands:
    updated.add NameMapEntry(
      command: command,
      tool: tool.name,
      locator: "bin/" & command,
    )
  saveNameMap(updated)

proc registerPackageCommands*(tool: ToolId) =
  var transaction = beginNameMapTransaction()
  defer: transaction.finishNameMapTransaction()
  transaction.registerPackageCommands(tool)
  transaction.commitNameMapTransaction()
