import std/[os, sets, strutils]

import ./[catalog, installer, resolver, shims, toolversions]

proc usage(): string =
  """Usage:
  selever install [-n|--dry-run]
  selever update  [-n|--dry-run]
  selever list
  selever ls

Install reuses complete local selectors. Update refreshes mutable selectors.
Shim execution never installs or accesses the network."""

proc parseDryRun(arguments: seq[string]): bool =
  if arguments.len == 0:
    return false
  if arguments.len == 1 and arguments[0] in ["-n", "--dry-run"]:
    return true
  raise newException(ValueError, "expected only -n or --dry-run")

proc applyVisible(mode: InstallMode, dryRun: bool): int =
  let selections = visibleSelections(getCurrentDir())
  if selections.len == 0:
    stderr.writeLine("selever: no " & ToolVersionsFile & " selections found")
    return 1
  let installed = installAll(
    selections,
    InstallOptions(mode: mode, dryRun: dryRun),
  )
  if not dryRun:
    syncShims(installed)
  0

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
  stdout.writeLine(".tool-versions")
  if documents.len == 0:
    stdout.writeLine("  (none)")
  var seen = initHashSet[string]()
  for document in documents:
    stdout.writeLine("  " & document.path)
    for selection in document.selections:
      let state =
        if selection.name in seen: "shadowed"
        else: "active"
      stdout.writeLine("    " & selection.name & " " &
        selection.versions.join(" ") & " [" & state & "]")
      seen.incl selection.name

  stdout.writeLine("active")
  if selections.len == 0:
    stdout.writeLine("  (none)")
  for selection in selections:
    stdout.writeLine("  " & selection.name & "  " &
      selection.selectionStatus & "  [" & selection.source & "]")
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
