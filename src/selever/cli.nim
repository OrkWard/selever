import std/[os, sets, strutils]

import ./[catalog, dispatch, installer, processutil, resolver, shims, toolversions]

proc usage*(): string =
  """Usage:
  selever install [-n|--dry-run]
  selever update  [-n|--dry-run]
  selever list
  selever ls
  selever exec -- <command> [arguments...]

Supported tools and selectors:
  nodejs        latest | lts | lts/<codename> | <version-prefix>
  golang        latest | <version-prefix>
  java          latest | lts | <feature>[.<minor>[.<security>[.<patch>]]][+<build>]
  zig           latest | <version-prefix>
  zls           latest | <version-prefix>
  npm:<package> <node-selector>:<package-selector>
  gopkg:<module> <go-selector>:<package-selector>

Runtime tools also accept system and path:<directory>.
Version prefixes select the latest matching release; exact versions are immutable.
Package selectors are passed unchanged to npm or go install.
Install reuses complete local selectors. Update refreshes mutable selectors.
Shim execution never installs or accesses the network.
Exec only sets JAVA_HOME for the selected JDK."""

proc parseDryRun(arguments: seq[string]): bool =
  if arguments.len == 0:
    return false
  if arguments.len == 1 and arguments[0] in ["-n", "--dry-run"]:
    return true
  raise newException(ValueError, "expected only -n or --dry-run")

proc applyVisible(mode: InstallMode, dryRun: bool): int =
  let selections = visibleSelections(getCurrentDir())
  if selections.len == 0:
    stderr.writeLine("selever: no tool version selections found")
    return 1
  let installed = installAll(
    selections,
    InstallOptions(mode: mode, dryRun: dryRun),
  )
  if not dryRun:
    syncShims(installed)
  0

proc execWithJava(arguments: seq[string]): int =
  if arguments.len < 2 or arguments[0] != "--":
    raise newException(ValueError,
      "expected exec -- <command> [arguments...]")
  let commandArguments =
    if arguments.len > 2: arguments[2 .. ^1]
    else: @[]
  runCommand(
    arguments[1],
    commandArguments,
    environmentWith("JAVA_HOME", resolveJavaHome(getCurrentDir())),
  )

proc selectionStatus(selection: ToolSelection): string =
  let tool = parseToolId(selection.name)
  if not tool.isSupported:
    return "unsupported"
  if tool.isRuntime:
    var states: seq[string]
    for selector in selection.versions:
      if selector == "system" or selector.startsWith("path:"):
        states.add selector & " external"
      else:
        let concrete = resolveRuntimeLocal(tool, selector)
        if concrete.len == 0:
          states.add selector & " missing"
        elif concrete == selector:
          states.add selector & " installed"
        else:
          states.add selector & " -> " & concrete & " installed"
    return states.join(", ")

  if selection.versions.len != 1:
    return "invalid: expected one runtime:package pair"
  let pair = parsePackagePair(selection.versions[0])
  let parent = tool.parentToolId
  let parentVersion = resolveRuntimeLocal(parent, pair.runtime)
  if parentVersion.len == 0:
    return pair.runtime & ":" & pair.package & " missing runtime"
  let packageVersion = resolvePackageLocal(tool, pair.package, parentVersion)
  if packageVersion.len == 0:
    return pair.runtime & " -> " & parentVersion & ", " &
      pair.package & " missing"
  pair.runtime & " -> " & parentVersion & ", " &
    pair.package & " -> " & packageVersion & " installed"

proc listVisible(): int =
  let documents = discoveredToolVersions(getCurrentDir())
  let selections = visibleSelections(getCurrentDir())
  stdout.writeLine("tool version files")
  if documents.len == 0:
    stdout.writeLine("  (none)")
  for document in documents:
    stdout.writeLine("  " & document.path)

  stdout.writeLine("active")
  if selections.len == 0:
    stdout.writeLine("  (none)")
  for selection in selections:
    stdout.writeLine("  " & selection.name & "  " &
      selection.selectionStatus & "  [" & selection.source & "]")

  var seen = initHashSet[string]()
  var shadowed: seq[ToolSelection]
  for document in documents:
    for selection in document.selections:
      if selection.name in seen:
        shadowed.add selection
      else:
        seen.incl selection.name
  if shadowed.len > 0:
    stdout.writeLine("shadowed")
    for selection in shadowed:
      stdout.writeLine("  " & selection.name & " " &
        selection.versions.join(" ") & "  [" & selection.source & "]")
  0

proc runCli*(arguments: seq[string]): int =
  try:
    if arguments.len == 0:
      stdout.writeLine(usage())
      return 0
    case arguments[0]
    of "install":
      return applyVisible(imInstall, parseDryRun(arguments[1 .. ^1]))
    of "update":
      return applyVisible(imUpdate, parseDryRun(arguments[1 .. ^1]))
    of "list", "ls":
      if arguments.len != 1:
        raise newException(ValueError, "list accepts no arguments")
      return listVisible()
    of "exec":
      return execWithJava(arguments[1 .. ^1])
    of "-h", "--help":
      if arguments.len != 1:
        raise newException(ValueError, "help accepts no arguments")
      stdout.writeLine(usage())
      return 0
    else:
      stderr.writeLine(usage())
      return 2
  except ValueError as error:
    stderr.writeLine("selever: " & error.msg)
    2
