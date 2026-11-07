## The environment `selever exec` gives a command: Java and the C toolchains
## have no shims, so their selections only take effect through it.

import std/[os, strutils]

import ./[catalog, dispatch, environment, layout, msvc, resolver, toolversions]

proc javaHome(executable: string): string =
  var path = absolutePath(executable)
  for _ in 0 ..< 16:
    if not symlinkExists(path):
      break
    let target = expandSymlink(path)
    path =
      if target.isAbsolute: target
      else: absolutePath(target, parentDir(path))
  let bin = parentDir(path)
  if splitPath(bin).tail.cmpIgnoreCase("bin") != 0:
    raise newException(ValueError,
      "selected java executable is not inside a JDK bin directory: " & executable)
  parentDir(bin)

proc javaEnvironment(selection: ToolSelection): EnvironmentChanges =
  let tool = parseToolId("java")
  var home = ""
  for selector in selection.versions:
    if selector == "system":
      let executable = systemExecutable("java")
      if executable.len > 0:
        home = javaHome(executable)
    elif selector.startsWith("path:"):
      let executable = pathExecutable(selector[5 .. ^1], "java")
      if executable.len > 0:
        home = javaHome(executable)
    else:
      let concrete = resolveRuntimeLocal(tool, selector)
      if concrete.len > 0:
        home = runtimeInstallDir(tool, concrete)
    if home.len > 0:
      break
  if home.len == 0:
    raise newException(ValueError,
      "java: no matching installed version in tool version files")
  result.prepend("PATH", home / "bin")
  result.setVariable("JAVA_HOME", home)

proc compilerEnvironment(root, cc, cxx: string): EnvironmentChanges =
  let suffix = when defined(windows): ".exe" else: ""
  let bin = root / "bin"
  result.prepend("PATH", bin)
  result.setVariable("CC", bin / (cc & suffix))
  result.setVariable("CXX", bin / (cxx & suffix))

proc toolchainEnvironment(tool: ToolId,
                          selection: ToolSelection): EnvironmentChanges =
  for selector in selection.versions:
    let concrete = resolveRuntimeLocal(tool, selector)
    if concrete.len == 0:
      continue
    let root = runtimeInstallDir(tool, concrete)
    case tool.kind
    of tkMsvc:
      return msvcEnvironment(root, parseMsvcSelector(selector))
    of tkGcc:
      return compilerEnvironment(root, "gcc", "g++")
    of tkLlvmMingw:
      return compilerEnvironment(root, "clang", "clang++")
    else:
      discard
  raise newException(ValueError,
    tool.name & ": no matching installed version in tool version files")

proc execEnvironment*(startDir: string): EnvironmentChanges =
  ## Earlier selections come first on PATH and the other lists.
  for selection in visibleSelections(startDir):
    let tool = parseToolId(selection.name)
    if tool.kind == tkJava:
      result.merge javaEnvironment(selection)
    elif tool.isCToolchain:
      result.merge toolchainEnvironment(tool, selection)

proc findCommand*(command, pathValue: string): string =
  ## The command as found on pathValue, which is not this process's PATH.
  if command.contains({'/', '\\'}):
    return command
  for directory in pathValue.split(PathSep):
    if directory.len == 0:
      continue
    let candidate = executableIn(directory, command)
    if candidate.len > 0:
      return candidate
  command
