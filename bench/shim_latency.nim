import std/[algorithm, cmdline, json, monotimes, osproc, streams, strutils, times]

proc usage(): string =
  "usage: shim_latency <direct> <shim> [iterations] [arguments ...]"

proc measure(executable: string, arguments: seq[string]): int64 =
  let started = getMonoTime()
  let process = startProcess(
    executable,
    args = arguments,
    options = {poStdErrToStdOut},
  )
  discard process.outputStream.readAll()
  let exitCode = process.waitForExit()
  process.close()
  if exitCode != 0:
    raise newException(OSError,
      executable & " exited with status " & $exitCode)
  (getMonoTime() - started).inNanoseconds

proc percentile(samples: var seq[int64], percent: int): int64 =
  samples.sort()
  samples[(samples.high * percent) div 100]

proc milliseconds(nanoseconds: int64): float =
  nanoseconds.float / 1_000_000.0

proc main(): int =
  let parameters = commandLineParams()
  if parameters.len < 2:
    stderr.writeLine(usage())
    return 2

  let direct = parameters[0]
  let shim = parameters[1]
  let iterations =
    if parameters.len >= 3: parseInt(parameters[2])
    else: 200
  if iterations < 10:
    raise newException(ValueError, "iterations must be at least 10")
  let arguments =
    if parameters.len >= 4: parameters[3 .. ^1]
    else: @["--version"]

  for unused in 0 ..< 10:
    discard unused
    discard measure(direct, arguments)
    discard measure(shim, arguments)

  var directSamples = newSeqOfCap[int64](iterations)
  var shimSamples = newSeqOfCap[int64](iterations)
  for index in 0 ..< iterations:
    if index mod 2 == 0:
      directSamples.add measure(direct, arguments)
      shimSamples.add measure(shim, arguments)
    else:
      shimSamples.add measure(shim, arguments)
      directSamples.add measure(direct, arguments)

  let directMedian = directSamples.percentile(50)
  let shimMedian = shimSamples.percentile(50)
  let directP95 = directSamples.percentile(95)
  let shimP95 = shimSamples.percentile(95)
  stdout.writeLine $(%*{
    "iterations": iterations,
    "directMedianMs": directMedian.milliseconds,
    "shimMedianMs": shimMedian.milliseconds,
    "overheadMedianMs": (shimMedian - directMedian).milliseconds,
    "directP95Ms": directP95.milliseconds,
    "shimP95Ms": shimP95.milliseconds,
    "overheadP95Ms": (shimP95 - directP95).milliseconds,
  })
  0

when isMainModule:
  quit(main())
