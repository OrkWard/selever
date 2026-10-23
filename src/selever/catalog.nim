import std/strutils

type
  ToolKind* = enum
    tkUnknown,
    tkNodejs,
    tkGolang,
    tkZig,
    tkZls,
    tkNpm,
    tkGoPackage

  ToolId* = object
    kind*: ToolKind
    name*: string
    package*: string

  PackagePair* = object
    runtime*: string
    package*: string

proc parseToolId*(name: string): ToolId =
  result.name = name
  case name
  of "nodejs":
    result.kind = tkNodejs
  of "golang":
    result.kind = tkGolang
  of "zig":
    result.kind = tkZig
  of "zls":
    result.kind = tkZls
  else:
    if name.startsWith("npm:") and name.len > 4:
      result.kind = tkNpm
      result.package = name[4 .. ^1]
    elif name.startsWith("gopkg:") and name.len > 6:
      result.kind = tkGoPackage
      result.package = name[6 .. ^1]

proc parentTool*(tool: ToolId): string =
  case tool.kind
  of tkNpm:
    "nodejs"
  of tkGoPackage:
    "golang"
  else:
    ""

proc parentToolId*(tool: ToolId): ToolId =
  parseToolId(tool.parentTool)

proc isSupported*(tool: ToolId): bool =
  tool.kind != tkUnknown

proc isPackage*(tool: ToolId): bool =
  tool.kind in {tkNpm, tkGoPackage}

proc isRuntime*(tool: ToolId): bool =
  tool.kind in {tkNodejs, tkGolang, tkZig, tkZls}

proc parsePackagePair*(value: string): PackagePair =
  let separator = value.find(':')
  if separator <= 0 or separator == value.high:
    raise newException(ValueError,
      "expected <runtime-selector>:<package-selector>, got " & value)
  result.runtime = value[0 ..< separator]
  result.package = value[separator + 1 .. ^1]
  if result.runtime.contains({' ', '\t', '\r', '\n'}) or
      result.package.contains({' ', '\t', '\r', '\n'}):
    raise newException(ValueError, "invalid package selector: " & value)
