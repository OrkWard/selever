import std/strutils

type
  ToolKind* = enum
    tkUnknown,
    tkNodejs,
    tkGolang,
    tkJava,
    tkZig,
    tkZls,
    tkMsvc,
    tkGcc,
    tkLlvmMingw,
    tkNpm,
    tkGoPackage

  ToolId* = object
    kind*: ToolKind
    name*: string
    package*: string

  PackagePair* = object
    runtime*: string
    package*: string

  MsvcSelector* = object
    toolset*: string
    target*: string

const MsvcTargets* = ["x64", "x86", "arm64"]

proc nativeArchitecture*(): string =
  when hostCPU == "amd64": "x64"
  elif hostCPU == "arm64": "arm64"
  elif hostCPU == "i386": "x86"
  else: raise newException(ValueError, "unsupported architecture: " & hostCPU)

proc parseToolId*(name: string): ToolId =
  result.name = name
  case name
  of "nodejs":
    result.kind = tkNodejs
  of "golang":
    result.kind = tkGolang
  of "java":
    result.kind = tkJava
  of "zig":
    result.kind = tkZig
  of "zls":
    result.kind = tkZls
  of "msvc":
    result.kind = tkMsvc
  of "gcc":
    result.kind = tkGcc
  of "llvm-mingw":
    result.kind = tkLlvmMingw
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

proc isCToolchain*(tool: ToolId): bool =
  tool.kind in {tkMsvc, tkGcc, tkLlvmMingw}

proc isRuntime*(tool: ToolId): bool =
  tool.kind in {tkNodejs, tkGolang, tkJava, tkZig, tkZls} or tool.isCToolchain

proc parseMsvcSelector*(selector: string): MsvcSelector =
  ## <toolset>[-<target>], e.g. 14.44 or 14.44-arm64; no target means native.
  let separator = selector.find('-')
  if separator < 0:
    result.toolset = selector
    result.target = nativeArchitecture()
  else:
    result.toolset = selector[0 ..< separator]
    result.target = selector[separator + 1 .. ^1]
  let parts = result.toolset.split('.')
  if parts.len != 2 or parts[0].len == 0 or parts[1].len == 0 or
      not result.toolset.allCharsInSet({'0' .. '9', '.'}):
    raise newException(ValueError,
      "msvc expects <toolset>[-<target>] such as 14.44 or 14.44-arm64, got " &
      selector)
  if result.target notin MsvcTargets:
    raise newException(ValueError,
      "msvc target must be one of " & MsvcTargets.join(", ") & ", got " &
      result.target)

proc storedSelector*(tool: ToolId, selector: string): string =
  ## The install directory name a selector refers to.
  if tool.kind == tkMsvc: parseMsvcSelector(selector).toolset
  else: selector

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
