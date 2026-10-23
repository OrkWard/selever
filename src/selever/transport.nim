import std/[os, strutils]

import ./processutil

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

proc extractArchive*(archive, destination: string; stripTopDir: bool) =
  let staging = destination & ".staging"
  removeDir(staging)
  createDir(staging)
  var arguments = @["-xf", archive, "-C", staging]
  if stripTopDir:
    arguments.add ["--strip-components", "1"]
  try:
    runChecked("tar", arguments)
    if dirExists(destination):
      removeDir(staging)
      return
    moveDir(staging, destination)
  except CatchableError:
    removeDir(staging)
    raise
