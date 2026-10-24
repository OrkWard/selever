import std/[os, osproc, strtabs]

when not defined(windows):
  import std/posix

proc currentEnvironment*(): StringTableRef =
  when defined(windows):
    result = newStringTable(modeCaseInsensitive)
  else:
    result = newStringTable(modeCaseSensitive)
  for key, value in envPairs():
    result[key] = value

proc environmentWith*(key, value: string): StringTableRef =
  result = currentEnvironment()
  result[key] = value

proc isExecutableFile*(path: string): bool =
  if not fileExists(path):
    return false
  when defined(windows):
    true
  else:
    posix.access(path.cstring, X_OK) == 0

proc runCommand*(command: string, arguments: openArray[string],
                 env: StringTableRef = nil): int =
  let process = startProcess(
    command,
    args = arguments,
    env = env,
    options = {poUsePath, poParentStreams},
  )
  result = process.waitForExit()
  process.close()

proc runChecked*(command: string, arguments: openArray[string],
                 env: StringTableRef = nil) =
  let exitCode = runCommand(command, arguments, env)
  if exitCode != 0:
    raise newException(OSError,
      command & " exited with status " & $exitCode)

proc captureCommand*(command: string, arguments: openArray[string],
                     env: StringTableRef = nil): string =
  execProcess(
    command,
    args = arguments,
    env = env,
    options = {poUsePath},
  )
