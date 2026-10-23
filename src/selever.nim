import std/os

when defined(windows):
  import std/strutils

import selever/[cli, dispatch]

proc invocationName(): string =
  let parts = splitFile(paramStr(0))
  when defined(windows):
    result = parts.name.toLowerAscii()
  else:
    result = parts.name

proc main(): int =
  let arguments = commandLineParams()
  try:
    if invocationName() == "selever":
      return runCli(arguments)

    let resolved = resolveCommand(invocationName(), getCurrentDir())
    return runResolved(resolved, arguments)
  except CatchableError as error:
    stderr.writeLine("selever: " & error.msg)
    return 1

when isMainModule:
  quit(main())
