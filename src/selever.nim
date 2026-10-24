import std/os

import selever/[cli, dispatch]

proc main(): int =
  let arguments = commandLineParams()
  try:
    if invocationName(paramStr(0)) == "selever":
      return runCli(arguments)

    let resolved = resolveCommand(invocationName(paramStr(0)), getCurrentDir())
    return runResolved(resolved, arguments)
  except CatchableError as error:
    stderr.writeLine("selever: " & error.msg)
    return 1

when isMainModule:
  quit(main())
