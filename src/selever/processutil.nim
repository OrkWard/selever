import std/[osproc, strtabs]

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
