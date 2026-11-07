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
    atl*: bool
    mfc*: bool
    mbcs*: bool
    sdk*: string  ## Windows SDK build, e.g. 26100; empty for the default.

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
  ## <toolset>[-<target>][+<option>...], e.g. 14.44, 14.44-arm64 or
  ## 14.44+mfc+sdk=26100; no target means native. Options: atl, mfc (with
  ## ATL), mbcs (MFC's multibyte libraries, with MFC) and sdk=<build>.
  let options = selector.split('+')
  let base = options[0]
  let separator = base.find('-')
  if separator < 0:
    result.toolset = base
    result.target = nativeArchitecture()
  else:
    result.toolset = base[0 ..< separator]
    result.target = base[separator + 1 .. ^1]
  let parts = result.toolset.split('.')
  if parts.len != 2 or parts[0].len == 0 or parts[1].len == 0 or
      not result.toolset.allCharsInSet({'0' .. '9', '.'}):
    raise newException(ValueError,
      "msvc expects <toolset>[-<target>][+<option>...] such as 14.44, " &
      "14.44-arm64 or 14.44+mfc, got " & selector)
  if result.target notin MsvcTargets:
    raise newException(ValueError,
      "msvc target must be one of " & MsvcTargets.join(", ") & ", got " &
      result.target)
  for option in options[1 .. ^1]:
    case option
    of "atl":
      result.atl = true
    of "mfc":
      result.atl = true
      result.mfc = true
    of "mbcs":
      result.atl = true
      result.mfc = true
      result.mbcs = true
    else:
      if option.startsWith("sdk=") and option.len > 4 and
          option[4 .. ^1].allCharsInSet(Digits):
        result.sdk = option[4 .. ^1]
      else:
        raise newException(ValueError,
          "msvc options are atl, mfc, mbcs and sdk=<build>, got " & option &
          " in " & selector)

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
