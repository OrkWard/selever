import std/[os, strutils]

when defined(windows):
  import std/osproc
else:
  import std/posix

import ./[catalog, layout, namemap, processutil, resolver, toolversions]

type
  ResolvedCommand* = object
    executable*: string
    arguments*: seq[string]
    tool*: string
    version*: string

proc invocationName*(path: string): string =
  when defined(windows):
    splitFile(path).name.toLowerAscii()
  else:
    splitPath(path).tail

proc executableIn(directory, command: string): string =
  when defined(windows):
    for extension in [".exe", ".com"]:
      let candidate = directory / (command & extension)
      if candidate.isExecutableFile:
        return candidate
  else:
    let candidate = directory / command
    if candidate.isExecutableFile:
      return candidate

proc isOwnDispatcher(path: string): bool =
  try:
    sameFile(path, getAppFilename())
  except OSError:
    false

proc systemExecutable(command: string): string =
  let ownShims = normalizedPath(shimDir())
  for directory in getEnv("PATH").split(PathSep):
    if directory.len == 0:
      continue
    if cmpPaths(normalizedPath(directory), ownShims) == 0:
      continue
    let candidate = executableIn(directory, command)
    if candidate.len > 0 and not candidate.isOwnDispatcher:
      return candidate

proc handlesBuiltinCommand(tool: ToolId, command: string): bool =
  case tool.kind
  of tkNodejs: command in ["node", "npm", "npx", "corepack"]
  of tkGolang: command in ["go", "gofmt"]
  of tkZig: command == "zig"
  of tkZls: command == "zls"
  else: false

proc nodeScript(root, command: string): string =
  let moduleRoot =
    when defined(windows): root / "node_modules"
    else: root / "lib" / "node_modules"
  case command
  of "npm": moduleRoot / "npm" / "bin" / "npm-cli.js"
  of "npx": moduleRoot / "npm" / "bin" / "npx-cli.js"
  of "corepack": moduleRoot / "corepack" / "dist" / "corepack.js"
  else: ""

proc installedBuiltin(tool: ToolId, version,
                      command: string): ResolvedCommand =
  let root = runtimeInstallDir(tool, version)
  if tool.kind == tkNodejs and command in ["npm", "npx", "corepack"]:
    let script = nodeScript(root, command)
    let node = runtimeExecutable(tool, root)
    if node.isExecutableFile and fileExists(script):
      return ResolvedCommand(
        executable: node,
        arguments: @[script],
        tool: tool.name,
        version: version,
      )

  for directory in executableDirs(tool, version):
    let executable = executableIn(directory, command)
    if executable.len > 0:
      return ResolvedCommand(
        executable: executable,
        tool: tool.name,
        version: version,
      )

proc pathExecutable(pathValue, command: string): string =
  let expanded = pathValue.expandTilde
  result = executableIn(expanded, command)
  if result.len == 0:
    result = executableIn(expanded / "bin", command)

proc stripEnvArguments(fields: seq[string]): seq[string] =
  var index = 0
  while index < fields.len:
    if fields[index] in ["-S", "--split-string"] or
        fields[index].contains('='):
      inc index
    elif fields[index].startsWith("-S") and fields[index].len > 2:
      result.add fields[index][2 .. ^1]
      inc index
      break
    else:
      break
  if index < fields.len:
    result.add fields[index .. ^1]

proc nodeShebang(path: string): tuple[isNode: bool, arguments: seq[string]] =
  var file: File
  if not open(file, path, fmRead):
    return
  defer: close(file)
  var line: string
  if not file.readLine(line) or not line.startsWith("#!"):
    return
  var fields = line[2 .. ^1].splitWhitespace()
  if fields.len == 0:
    return
  if splitFile(fields[0]).name == "env":
    fields = stripEnvArguments(fields[1 .. ^1])
  if fields.len > 0 and splitFile(fields[0]).name == "node":
    result.isNode = true
    if fields.len > 1:
      result.arguments = fields[1 .. ^1]

proc resolvePackageCommand(command, startDir: string,
                           entry: NameMapEntry): ResolvedCommand =
  let selections = visibleSelections(startDir)
  let index = selections.findSelection(entry.tool)
  if index < 0:
    raise newException(ValueError,
      command & ": " & entry.tool & " is not configured in " & ToolVersionsFile)
  let selection = selections[index]
  if selection.versions.len != 1:
    raise newException(ValueError,
      entry.tool & " requires exactly one <runtime>:<package> pair")
  let pair = parsePackagePair(selection.versions[0])
  let tool = parseToolId(entry.tool)
  let parent = tool.parentToolId
  let parentVersion = resolveRuntimeLocal(parent, pair.runtime)
  if parentVersion.len == 0:
    raise newException(ValueError,
      command & ": runtime is not installed: " & parent.name & " " & pair.runtime)
  let packageVersion = resolvePackageLocal(tool, pair.package, parentVersion)
  if packageVersion.len == 0:
    raise newException(ValueError,
      command & ": package is not installed: " & entry.tool & " " & pair.package)

  let packageDir = packageInstallDir(tool, packageVersion, parentVersion)
  var executable = packageDir / entry.locator
  when defined(windows):
    if not executable.isExecutableFile:
      executable = executableIn(parentDir(executable), command)
  if not executable.isExecutableFile:
    raise newException(ValueError,
      command & ": package does not provide this command for the selected version")

  if tool.kind == tkNpm:
    let shebang = nodeShebang(executable)
    if shebang.isNode:
      let node = runtimeExecutable(parent, runtimeInstallDir(parent, parentVersion))
      return ResolvedCommand(
        executable: node,
        arguments: shebang.arguments & @[executable],
        tool: tool.name,
        version: pair.package,
      )

  ResolvedCommand(
    executable: executable,
    tool: tool.name,
    version: pair.package,
  )

proc resolveCommand*(command, startDir: string): ResolvedCommand =
  let mapped = lookupName(command)
  if mapped.command.len > 0:
    return resolvePackageCommand(command, startDir, mapped)

  let selections = visibleSelections(startDir)
  for selection in selections:
    let tool = parseToolId(selection.name)
    if not tool.isRuntime or not tool.handlesBuiltinCommand(command):
      continue
    for selector in selection.versions:
      if selector == "system":
        let executable = systemExecutable(command)
        if executable.len > 0:
          return ResolvedCommand(
            executable: executable,
            tool: selection.name,
            version: selector,
          )
        continue
      if selector.startsWith("path:"):
        let executable = pathExecutable(selector[5 .. ^1], command)
        if executable.len > 0 and not executable.isOwnDispatcher:
          return ResolvedCommand(
            executable: executable,
            tool: selection.name,
            version: selector,
          )
        continue

      let concrete = resolveRuntimeLocal(tool, selector)
      if concrete.len == 0:
        continue
      result = installedBuiltin(tool, concrete, command)
      if result.executable.len > 0:
        result.version = selector
        return

  raise newException(ValueError,
    command & ": no matching installed version in " & ToolVersionsFile)

proc runResolved*(resolved: ResolvedCommand,
                  arguments: seq[string]): int =
  when defined(windows):
    let process = startProcess(
      resolved.executable,
      args = resolved.arguments & arguments,
      options = {poParentStreams},
    )
    result = process.waitForExit()
    process.close()
  else:
    let args = @[resolved.executable] & resolved.arguments & arguments
    let rawArgs = allocCStringArray(args)
    discard execv(resolved.executable.cstring, rawArgs)
    deallocCStringArray(rawArgs)
    raiseOSError(osLastError())
