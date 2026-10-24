import std/[algorithm, json, os, strutils]

import ./[catalog, layout, processutil, transport]

const
  NodeIndexUrl = "https://nodejs.org/dist/index.json"
  GoIndexUrl = "https://go.dev/dl/?mode=json&include=all"
  JavaApiBase* = "https://api.adoptium.net/v3"
  JavaPageSize* = 50
  ZigIndexUrl* = "https://ziglang.org/download/index.json"
  ZlsIndexUrl* = "https://releases.zigtools.org/v1/zls/index.json"

proc runtimeExecutable*(tool: ToolId, root: string): string =
  let suffix = when defined(windows): ".exe" else: ""
  case tool.kind
  of tkNodejs:
    when defined(windows): root / "node.exe"
    else: root / "bin" / "node"
  of tkGolang:
    root / "bin" / ("go" & suffix)
  of tkJava:
    root / "bin" / ("java" & suffix)
  of tkZig:
    root / ("zig" & suffix)
  of tkZls:
    root / ("zls" & suffix)
  else:
    ""

proc runtimeComplete*(tool: ToolId, version: string): bool =
  let root = runtimeInstallDir(tool, version)
  let executable = runtimeExecutable(tool, root)
  if executable.len == 0 or not executable.isExecutableFile:
    return false
  if tool.kind == tkJava:
    let suffix = when defined(windows): ".exe" else: ""
    return (root / "bin" / ("javac" & suffix)).isExecutableFile
  true

proc javaPlatform*(): string =
  when defined(windows): "windows"
  elif defined(macosx): "mac"
  elif defined(linux): "linux"
  else: raise newException(ValueError, "unsupported Java platform")

proc javaArchitecture*(): string =
  when hostCPU == "amd64": "x64"
  elif hostCPU == "arm64": "aarch64"
  else: raise newException(ValueError, "unsupported Java architecture: " & hostCPU)

proc javaFeatureReleasesUrl*(feature, page: int): string =
  JavaApiBase & "/assets/feature_releases/" & $feature & "/ga" &
    "?architecture=" & javaArchitecture() &
    "&heap_size=normal&image_type=jdk&jvm_impl=hotspot&os=" & javaPlatform() &
    "&page=" & $page & "&page_size=" & $JavaPageSize &
    "&project=jdk&sort_order=DESC&vendor=eclipse"

proc javaVersion*(release: JsonNode): string =
  let version =
    if release.hasKey("version_data"): release["version_data"]
    else: release["version"]
  var parts = @[$version["major"].getInt()]
  if version.hasKey("minor"):
    parts.add $version["minor"].getInt()
  if version.hasKey("security"):
    parts.add $version["security"].getInt()
  if version.hasKey("patch") and version["patch"].getInt() > 0:
    parts.add $version["patch"].getInt()
  result = parts.join(".")
  if version.hasKey("build"):
    result.add "+" & $version["build"].getInt()

proc packageComplete*(tool: ToolId, version, parentVersion: string): bool =
  let binDir = packageInstallDir(tool, version, parentVersion) / "bin"
  if not dirExists(binDir):
    return false
  for kind, path in walkDir(binDir):
    if kind in {pcFile, pcLinkToFile} and path.isExecutableFile:
      return true

proc resolvedLinkVersion(path, versionsRoot: string): string =
  if not symlinkExists(path):
    return
  let storedTarget = expandSymlink(path)
  let target =
    if storedTarget.isAbsolute: storedTarget
    else: absolutePath(storedTarget, parentDir(path))
  let normalizedTarget = normalizedPath(target)
  if cmpPaths(parentDir(normalizedTarget), normalizedPath(versionsRoot)) != 0:
    return
  result = unfsComponent(splitPath(normalizedTarget).tail)

proc resolveRuntimeLocal*(tool: ToolId, selector: string): string =
  let path = runtimeSelectorPath(tool, selector)
  if symlinkExists(path):
    result = resolvedLinkVersion(path, runtimeRoot(tool))
  elif dirExists(path):
    result = selector
  if result.len > 0 and not runtimeComplete(tool, result):
    result.setLen(0)

proc resolvePackageLocal*(tool: ToolId, selector,
                          parentVersion: string): string =
  result = resolvedLinkVersion(
    packageSelectorPath(tool, selector, parentVersion),
    packageVersionsRoot(tool, parentVersion),
  )
  if result.len > 0 and not packageComplete(tool, result, parentVersion):
    result.setLen(0)

proc numericParts(value: string): seq[int] =
  let core = value.split({'-', '+'}, maxsplit = 1)[0]
  for part in core.split('.'):
    if part.len == 0 or not part.allCharsInSet({'0' .. '9'}):
      return @[]
    result.add parseInt(part)

proc isConcreteVersion*(value: string): bool =
  numericParts(value).len >= 3

proc compareVersions(a, b: string): int =
  let left = numericParts(a)
  let right = numericParts(b)
  if left.len == 0 or right.len == 0:
    return cmp(a, b)
  for index in 0 ..< max(left.len, right.len):
    let l = if index < left.len: left[index] else: 0
    let r = if index < right.len: right[index] else: 0
    if l != r:
      return cmp(l, r)
  let leftPrerelease = a.contains('-')
  let rightPrerelease = b.contains('-')
  if leftPrerelease != rightPrerelease:
    return if leftPrerelease: -1 else: 1
  cmp(a, b)

proc numericPrefixes(version: string): seq[string] =
  let parts = numericParts(version)
  if parts.len < 2:
    return
  for count in 1 ..< parts.len:
    var fields: seq[string]
    for index in 0 ..< count:
      fields.add $parts[index]
    result.add fields.join(".")

proc replaceLink(path, target: string) =
  createDir(parentDir(path))
  let temporary = path & ".tmp-" & $getCurrentProcessId()
  if fileExists(temporary) or symlinkExists(temporary):
    removeFile(temporary)
  createSymlink(relativePath(target, parentDir(path)), temporary)
  if (fileExists(path) or dirExists(path)) and not symlinkExists(path):
    removeFile(temporary)
    raise newException(ValueError,
      "refusing to replace non-link selector: " & path)
  moveFile(temporary, path)

proc updateNumericLink(tool: ToolId, selector, concrete: string) =
  let existing = resolveRuntimeLocal(tool, selector)
  if existing.len > 0 and compareVersions(existing, concrete) >= 0:
    return
  replaceLink(
    runtimeSelectorPath(tool, selector),
    runtimeInstallDir(tool, concrete),
  )

proc createRuntimeLinks*(tool: ToolId, selector, concrete: string) =
  for prefix in numericPrefixes(concrete):
    updateNumericLink(tool, prefix, concrete)
  if selector != concrete:
    replaceLink(
      runtimeSelectorPath(tool, selector),
      runtimeInstallDir(tool, concrete),
    )

proc createPackageLink*(tool: ToolId, selector, concrete,
                        parentVersion: string) =
  replaceLink(
    packageSelectorPath(tool, selector, parentVersion),
    packageInstallDir(tool, concrete, parentVersion),
  )

proc matchesNumericSelector(version, selector: string): bool =
  let query = numericParts(selector)
  let candidate = numericParts(version)
  if query.len == 0 or query.len > candidate.len:
    return false
  for index, value in query:
    if candidate[index] != value:
      return false
  true

proc resolveNode(selector: string): string =
  if selector.startsWith('v'):
    raise newException(ValueError,
      "nodejs selectors must not use a leading v: " & selector)
  if selector.isConcreteVersion:
    return selector

  let index = parseJson(fetchText(NodeIndexUrl))
  let lower = selector.toLowerAscii()
  for release in index.items:
    var version = release["version"].getStr()
    if version.startsWith('v'):
      version = version[1 .. ^1]
    if version.contains('-'):
      continue
    var matches = false
    if lower == "latest":
      matches = true
    elif lower == "lts":
      matches = release.hasKey("lts") and release["lts"].kind == JString
    elif lower.startsWith("lts/"):
      matches = release.hasKey("lts") and release["lts"].kind == JString and
        release["lts"].getStr().cmpIgnoreCase(selector[4 .. ^1]) == 0
    else:
      matches = version.matchesNumericSelector(selector)
    if matches:
      return version
  raise newException(ValueError, "nodejs release not found: " & selector)

proc resolveGo(selector: string): string =
  if selector.startsWith("go"):
    raise newException(ValueError,
      "golang selectors must not use a leading go: " & selector)
  if selector.isConcreteVersion:
    return selector

  let index = parseJson(fetchText(GoIndexUrl))
  for release in index.items:
    var version = release["version"].getStr()
    if version.startsWith("go"):
      version = version[2 .. ^1]
    if selector == "latest" or version.matchesNumericSelector(selector):
      return version
  raise newException(ValueError, "golang release not found: " & selector)

proc normalizedJavaSelector(selector: string): string =
  result = selector
  for prefix in ["temurin-", "jdk-"]:
    if result.startsWith(prefix):
      result = result[prefix.len .. ^1]
      break

proc javaFeature(selector: string): int =
  let lower = selector.toLowerAscii()
  if lower in ["latest", "lts"]:
    let info = parseJson(fetchText(JavaApiBase & "/info/available_releases"))
    return info[
      if lower == "lts": "most_recent_lts"
      else: "most_recent_feature_release"
    ].getInt()
  let parts = numericParts(selector)
  if parts.len == 0:
    raise newException(ValueError, "invalid Java selector: " & selector)
  parts[0]

proc resolveJava(selector: string): string =
  let normalized = normalizedJavaSelector(selector)
  if normalized.contains('+'):
    return normalized
  let feature = javaFeature(normalized)
  for page in 0 .. 99:
    let releases = parseJson(fetchText(javaFeatureReleasesUrl(feature, page)))
    if releases.len == 0:
      break
    for release in releases.items:
      let version = javaVersion(release)
      if normalized.toLowerAscii() in ["latest", "lts"] or
          version.matchesNumericSelector(normalized):
        return version
    if releases.len < JavaPageSize:
      break
  raise newException(ValueError, "Java release not found: " & selector)

proc resolveZigStyle(selector, indexUrl: string): string =
  if selector.isConcreteVersion:
    return selector
  let index = parseJson(fetchText(indexUrl))
  var matches: seq[string]
  for name, release in index.pairs:
    let version =
      if name == "master" and release.hasKey("version"):
        release["version"].getStr()
      else:
        name
    if selector == "latest" or version.matchesNumericSelector(selector):
      matches.add version
  if matches.len == 0:
    raise newException(ValueError, "release not found: " & selector)
  matches.sort(proc(a, b: string): int = compareVersions(b, a))
  matches[0]

proc resolveInstallVersion*(tool: ToolId, selector: string): string =
  if selector.len == 0:
    raise newException(ValueError, "empty version selector for " & tool.name)
  case tool.kind
  of tkNodejs:
    resolveNode(selector)
  of tkGolang:
    resolveGo(selector)
  of tkJava:
    resolveJava(selector)
  of tkZig:
    resolveZigStyle(selector, ZigIndexUrl)
  of tkZls:
    resolveZigStyle(selector, ZlsIndexUrl)
  else:
    raise newException(ValueError, "not a runtime: " & tool.name)
