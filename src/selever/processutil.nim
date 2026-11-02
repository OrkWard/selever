import std/[os, osproc, strtabs]

when defined(windows):
  const
    CtrlCEvent = 0'u32
    CtrlBreakEvent = 1'u32

  proc setConsoleCtrlHandler(
    handler: proc(ctrlType: uint32): int32 {.stdcall, gcsafe.},
    add: int32,
  ): int32 {.stdcall, dynlib: "kernel32", importc: "SetConsoleCtrlHandler".}

  proc interruptHandler(ctrlType: uint32): int32 {.stdcall, gcsafe.} =
    if ctrlType in [CtrlCEvent, CtrlBreakEvent]: 1 else: 0
else:
  import std/posix

proc ignoreConsoleInterrupts*() =
  ## Keeps this process alive on Ctrl+C and Ctrl+Break while it waits for a
  ## child: the console sends them to the child too, and the child decides.
  ## Handlers are not inherited, so the child keeps the default behavior.
  when defined(windows):
    if setConsoleCtrlHandler(interruptHandler, 1) == 0:
      raiseOSError(osLastError())

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
  if exitCode != QuitSuccess:
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
