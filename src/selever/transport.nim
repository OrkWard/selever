import std/[os, osproc, strutils]

import ./processutil

type Download* = object
  url*: string
  destination*: string
  sha256*: string

const ParallelDownloads = 8

const CurlArguments = @[
  "--disable",
  "--fail",
  "--location",
  "--proto", "=https",
  "--proto-redir", "=https",
  "--retry", "3",
  "--show-error",
  "--silent",
]

proc fetchText*(url: string): string =
  result = captureCommand("curl", CurlArguments & @[url])

proc download*(url, destination: string) =
  if fileExists(destination):
    return
  createDir(parentDir(destination))
  let partial = destination & ".part"
  if fileExists(partial):
    removeFile(partial)
  try:
    runChecked("curl", CurlArguments & @["--output", partial, url])
    moveFile(partial, destination)
  except CatchableError:
    if fileExists(partial):
      removeFile(partial)
    raise

proc isSha256(value: string): bool =
  if value.len != 64:
    return false
  for ch in value:
    if ch notin {'0' .. '9', 'a' .. 'f', 'A' .. 'F'}:
      return false
  true

proc sha256File*(path: string): string =
  when defined(windows):
    let output = captureCommand("certutil", ["-hashfile", path, "SHA256"])
    for line in output.splitLines():
      let candidate = line.strip().replace(" ", "")
      if candidate.isSha256:
        return candidate.toLowerAscii()
  elif defined(macosx):
    let output = captureCommand("shasum", ["-a", "256", path])
    let candidate = output.splitWhitespace()[0]
    if candidate.isSha256:
      return candidate.toLowerAscii()
  else:
    let output = captureCommand("sha256sum", [path])
    let candidate = output.splitWhitespace()[0]
    if candidate.isSha256:
      return candidate.toLowerAscii()
  raise newException(ValueError, "could not calculate SHA-256 for " & path)

proc verifySha256*(path, expected: string) =
  if not expected.isSha256:
    raise newException(ValueError, "invalid expected SHA-256 for " & path)
  let actual = sha256File(path)
  if actual != expected.toLowerAscii():
    raise newException(ValueError,
      "SHA-256 mismatch for " & path & ": got " & actual &
      ", expected " & expected.toLowerAscii())

proc verifiedDownload*(url, destination, expected: string) =
  for attempt in 0 .. 1:
    download(url, destination)
    try:
      verifySha256(destination, expected)
      return
    except ValueError:
      if fileExists(destination):
        removeFile(destination)
      if attempt == 1:
        raise

proc verifiedDownloadAll*(downloads: openArray[Download]) =
  ## verifiedDownload for many files, ParallelDownloads at a time.
  var queue: seq[tuple[item: Download, retried: bool]]
  for item in downloads:
    if fileExists(item.destination):
      try:
        verifySha256(item.destination, item.sha256)
        continue
      except ValueError:
        removeFile(item.destination)
    var queued = false
    for entry in queue:
      if entry.item.destination == item.destination:
        queued = true
    if not queued:
      queue.add (item, false)

  var running: seq[tuple[process: Process, item: Download, retried: bool]]
  var failures: seq[string]
  var next = 0
  while next < queue.len or running.len > 0:
    while next < queue.len and running.len < ParallelDownloads:
      let (item, retried) = queue[next]
      inc next
      createDir(parentDir(item.destination))
      let partial = item.destination & ".part"
      if fileExists(partial):
        removeFile(partial)
      let process = startProcess("curl",
        args = CurlArguments & @["--output", partial, item.url],
        options = {poUsePath, poParentStreams})
      running.add (process, item, retried)

    var finished = false
    var index = 0
    while index < running.len:
      let (process, item, retried) = running[index]
      let exitCode = process.peekExitCode()
      if exitCode == -1:
        inc index
        continue
      process.close()
      running.delete(index)
      finished = true
      let partial = item.destination & ".part"
      if exitCode != QuitSuccess:
        if fileExists(partial):
          removeFile(partial)
        failures.add item.url & ": curl exited with status " & $exitCode
        continue
      try:
        verifySha256(partial, item.sha256)
        moveFile(partial, item.destination)
      except ValueError as error:
        removeFile(partial)
        if retried:
          failures.add error.msg
        else:
          queue.add (item, true)
    if not finished:
      sleep(50)

  if failures.len > 0:
    raise newException(OSError, "download failed:\n" & failures.join("\n"))

proc extractArchive*(archive, destination: string; stripTopDir: bool;
                     nestedRoot = "") =
  let staging = destination & ".staging"
  let publication = destination & ".publication"
  removeDir(staging)
  removeDir(publication)
  createDir(staging)
  var arguments = @["-xf", archive, "-C", staging]
  if stripTopDir:
    arguments.add ["--strip-components", "1"]
  try:
    runChecked("tar", arguments)
    if dirExists(destination):
      removeDir(staging)
      return
    if nestedRoot.len > 0:
      let nested = staging / nestedRoot
      if not dirExists(nested):
        raise newException(ValueError,
          "archive does not contain expected directory: " & nestedRoot)
      moveDir(nested, publication)
      removeDir(staging)
      moveDir(publication, destination)
    else:
      moveDir(staging, destination)
  except CatchableError:
    removeDir(staging)
    removeDir(publication)
    raise
