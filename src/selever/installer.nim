import std/[json, os, strtabs, strutils, tables, uri]

import ./[
  catalog,
  environment,
  execenv,
  layout,
  lockfile,
  luarocks,
  msvc,
  namemap,
  processutil,
  resolver,
  toolversions,
  transport,
]

const
  NodeIndexBase = "https://nodejs.org/dist"
  GoIndexUrl = "https://go.dev/dl/?mode=json&include=all"

# update refreshes selectors; install reuses every complete local selector.
type
  InstallMode* = enum
    imInstall,
    imUpdate

  InstallOptions* = object
    mode*: InstallMode
    dryRun*: bool
    ## Where the tool version files are; luabin builds C modules with the
    ## C toolchain selected there. Empty means the current directory.
    startDir*: string

  InstalledSelection* = object
    tool*: ToolId
    request*: string
    version*: string
    parentVersion*: string

proc archiveExtension(): string =
  when defined(windows): ".zip"
  else: ".tar.gz"

proc nodePlatform(): string =
  when defined(windows): "win"
  elif defined(macosx): "darwin"
  elif defined(linux): "linux"
  else: raise newException(ValueError, "unsupported Node.js platform")

proc nodeArchitecture(): string =
  when hostCPU == "amd64": "x64"
  elif hostCPU == "arm64": "arm64"
  else: raise newException(ValueError, "unsupported Node.js architecture: " & hostCPU)

proc goPlatform(): string =
  when defined(windows): "windows"
  elif defined(macosx): "darwin"
  elif defined(linux): "linux"
  else: raise newException(ValueError, "unsupported Go platform")

proc goArchitecture(): string =
  when hostCPU == "amd64": "amd64"
  elif hostCPU == "arm64": "arm64"
  else: raise newException(ValueError, "unsupported Go architecture: " & hostCPU)

proc zigPlatform(): string =
  let architecture =
    when hostCPU == "amd64": "x86_64"
    elif hostCPU == "arm64": "aarch64"
    else: raise newException(ValueError, "unsupported Zig architecture: " & hostCPU)
  let platform =
    when defined(windows): "windows"
    elif defined(macosx): "macos"
    elif defined(linux): "linux"
    else: raise newException(ValueError, "unsupported Zig platform")
  architecture & "-" & platform

proc nodeBin(root: string): string =
  when defined(windows): root
  else: root / "bin"

proc nodeExecutable(root: string): string =
  runtimeExecutable(parseToolId("nodejs"), root)

proc goExecutable(root: string): string =
  runtimeExecutable(parseToolId("golang"), root)

proc cachedText(url, namespace, name: string): string =
  let path = cacheRoot() / namespace / name
  download(url, path)
  readFile(path)

proc checksumFromLines(content, archiveName: string): string =
  for line in content.splitLines():
    let fields = line.splitWhitespace()
    if fields.len >= 2 and fields[^1].strip(chars = {'*'}) == archiveName:
      return fields[0]
  raise newException(ValueError, "checksum not found for " & archiveName)

proc installArchive(tool: ToolId, version, url, archiveName,
                    checksum: string; stripTopDir: bool;
                    nestedRoot = ""): string =
  result = runtimeInstallDir(tool, version)
  if runtimeComplete(tool, version):
    return
  let archive = cacheRoot() / tool.name / archiveName
  verifiedDownload(url, archive, checksum)
  createDir(parentDir(result))
  extractArchive(archive, result, stripTopDir, nestedRoot)
  if not runtimeComplete(tool, version):
    raise newException(ValueError,
      tool.name & " " & version & " did not contain its executable")

proc installNode(version: string): string =
  let tool = parseToolId("nodejs")
  result = runtimeInstallDir(tool, version)
  if runtimeComplete(tool, version):
    return
  var lock = acquireInstallLock(result & ".lock")
  defer: lock.release()
  if runtimeComplete(tool, version):
    return
  let archiveName = "node-v" & version & "-" & nodePlatform() & "-" &
    nodeArchitecture() & archiveExtension()
  let checksumText = cachedText(
    NodeIndexBase & "/v" & version & "/SHASUMS256.txt",
    "node",
    "SHASUMS256-" & fsComponent(version) & ".txt",
  )
  let checksum = checksumFromLines(checksumText, archiveName)
  result = installArchive(
    tool,
    version,
    NodeIndexBase & "/v" & version & "/" & archiveName,
    archiveName,
    checksum,
    stripTopDir = true,
  )

proc goChecksum(index: JsonNode, version, archiveName: string): string =
  for release in index.items:
    if release["version"].getStr() != "go" & version:
      continue
    for artifact in release["files"].items:
      if artifact["filename"].getStr() == archiveName:
        return artifact["sha256"].getStr()
  raise newException(ValueError, "checksum not found for " & archiveName)

proc installGo(version: string): string =
  let tool = parseToolId("golang")
  result = runtimeInstallDir(tool, version)
  if runtimeComplete(tool, version):
    return
  var lock = acquireInstallLock(result & ".lock")
  defer: lock.release()
  if runtimeComplete(tool, version):
    return
  let archiveName = "go" & version & "." & goPlatform() & "-" &
    goArchitecture() & archiveExtension()
  let indexText = cachedText(
    GoIndexUrl,
    "go",
    "index-" & fsComponent(version) & ".json",
  )
  let checksum = goChecksum(parseJson(indexText), version, archiveName)
  result = installArchive(
    tool,
    version,
    "https://go.dev/dl/" & archiveName,
    archiveName,
    checksum,
    stripTopDir = true,
  )

proc javaArtifact(version: string): tuple[url, name, checksum: string] =
  let feature = parseInt(version.split({'.', '+'}, maxsplit = 1)[0])
  for page in 0 .. 99:
    let indexText = cachedText(
      javaFeatureReleasesUrl(feature, page),
      "java",
      "feature-" & $feature & "-" & fsComponent(version) & "-" &
        $page & ".json",
    )
    let releases = parseJson(indexText)
    if releases.len == 0:
      break
    for release in releases.items:
      if javaVersion(release) != version:
        continue
      let binary =
        if release.hasKey("binary"): release["binary"]
        else: release["binaries"][0]
      let package = binary["package"]
      return (
        package["link"].getStr(),
        package["name"].getStr(),
        package["checksum"].getStr(),
      )
    if releases.len < JavaPageSize:
      break
  raise newException(ValueError, "Java release not found: " & version)

proc installJava(version: string): string =
  let tool = parseToolId("java")
  result = runtimeInstallDir(tool, version)
  if runtimeComplete(tool, version):
    return
  var lock = acquireInstallLock(result & ".lock")
  defer: lock.release()
  if runtimeComplete(tool, version):
    return
  let artifact = javaArtifact(version)
  result = installArchive(
    tool,
    version,
    artifact.url,
    artifact.name,
    artifact.checksum,
    stripTopDir = true,
    nestedRoot = when defined(macosx): "Contents/Home" else: "",
  )

proc zigArtifact(index: JsonNode, version,
                 platform: string): tuple[url, checksum: string] =
  var release: JsonNode
  if index.hasKey(version):
    release = index[version]
  elif index.hasKey("master") and index["master"]["version"].getStr() == version:
    release = index["master"]
  else:
    raise newException(ValueError, "release not found: " & version)
  if not release.hasKey(platform):
    raise newException(ValueError,
      "release " & version & " has no artifact for " & platform)
  let artifact = release[platform]
  (artifact["tarball"].getStr(), artifact["shasum"].getStr())

proc installZigStyle(toolName, version, indexUrl: string;
                     stripTopDir: bool): string =
  let tool = parseToolId(toolName)
  result = runtimeInstallDir(tool, version)
  if runtimeComplete(tool, version):
    return
  var lock = acquireInstallLock(result & ".lock")
  defer: lock.release()
  if runtimeComplete(tool, version):
    return
  let indexText = cachedText(
    indexUrl,
    toolName,
    "index-" & fsComponent(version) & ".json",
  )
  let artifact = zigArtifact(parseJson(indexText), version, zigPlatform())
  let archiveName = artifact.url.rsplit('/', maxsplit = 1)[^1]
  result = installArchive(
    tool,
    version,
    artifact.url,
    archiveName,
    artifact.checksum,
    stripTopDir,
  )

proc luaArtifact*(index: JsonNode, version,
                  platform: string): tuple[url, checksum: string] =
  let files = index{version, "files"}
  if files.isNil or files.kind != JObject:
    raise newException(ValueError, "lua release not found: " & version)
  if not files.hasKey(platform):
    raise newException(ValueError,
      "lua " & version & " has no bundle for " & platform)
  (files[platform]["url"].getStr(), files[platform]["sha256"].getStr())

proc installLua(version: string): string =
  let tool = parseToolId("lua")
  result = runtimeInstallDir(tool, version)
  if runtimeComplete(tool, version):
    return
  var lock = acquireInstallLock(result & ".lock")
  defer: lock.release()
  if runtimeComplete(tool, version):
    return
  let indexText = cachedText(
    LuaIndexUrl,
    "lua",
    "index-" & fsComponent(version) & ".json",
  )
  let artifact = luaArtifact(parseJson(indexText), version, luaPlatform())
  result = installArchive(
    tool,
    version,
    artifact.url,
    artifact.url.rsplit('/', maxsplit = 1)[^1],
    artifact.checksum,
    stripTopDir = true,
  )

proc githubReleaseAsset(repository, tag: string,
                        matches: proc(name: string): bool):
                        tuple[url, name, checksum: string] =
  ## The single asset of a release accepted by matches, with the SHA-256
  ## digest GitHub publishes for it.
  let response = fetchText(
    "https://api.github.com/repos/" & repository & "/releases/tags/" &
    encodeUrl(tag, usePlus = false))
  if response.len == 0:
    raise newException(ValueError,
      repository & " has no release " & tag)
  for asset in parseJson(response)["assets"].items:
    let name = asset["name"].getStr()
    if not matches(name):
      continue
    if result.name.len > 0:
      raise newException(ValueError,
        repository & " " & tag & " has more than one matching asset")
    let digest = asset{"digest"}.getStr()
    if not digest.startsWith("sha256:"):
      raise newException(ValueError, name & " has no SHA-256 digest")
    result = (asset["browser_download_url"].getStr(), name, digest[7 .. ^1])
  if result.name.len == 0:
    raise newException(ValueError,
      repository & " " & tag & " has no asset for this platform")

proc installGcc(tag: string): string =
  ## A winlibs release, named by its tag; the plain x86_64 build.
  if nativeArchitecture() != "x64":
    raise newException(ValueError, "gcc (winlibs) supports only x64 hosts")
  let tool = parseToolId("gcc")
  result = runtimeInstallDir(tool, tag)
  if runtimeComplete(tool, tag):
    return
  var lock = acquireInstallLock(result & ".lock")
  defer: lock.release()
  if runtimeComplete(tool, tag):
    return
  let asset = githubReleaseAsset("brechtsanders/winlibs_mingw", tag,
    proc(name: string): bool =
      name.startsWith("winlibs-x86_64-") and name.endsWith(".zip") and
        "-llvm-" notin name)
  result = installArchive(tool, tag, asset.url, asset.name, asset.checksum,
    stripTopDir = true)

proc installLlvmMingw(tag: string): string =
  ## An llvm-mingw release, named by its tag; always the ucrt build.
  let tool = parseToolId("llvm-mingw")
  result = runtimeInstallDir(tool, tag)
  if runtimeComplete(tool, tag):
    return
  var lock = acquireInstallLock(result & ".lock")
  defer: lock.release()
  if runtimeComplete(tool, tag):
    return
  let architecture =
    case nativeArchitecture()
    of "x64": "x86_64"
    of "arm64": "aarch64"
    else: "i686"
  let expected = "llvm-mingw-" & tag & "-ucrt-" & architecture & ".zip"
  let asset = githubReleaseAsset("mstorsjo/llvm-mingw", tag,
    proc(name: string): bool = name == expected)
  result = installArchive(tool, tag, asset.url, asset.name, asset.checksum,
    stripTopDir = true)

proc installRuntime(tool: ToolId, version: string): string =
  if tool.isCToolchain and not defined(windows):
    raise newException(ValueError, tool.name & " is only supported on Windows")
  case tool.kind
  of tkNodejs: installNode(version)
  of tkGolang: installGo(version)
  of tkJava: installJava(version)
  of tkZig: installZigStyle("zig", version, ZigIndexUrl, stripTopDir = true)
  of tkZls: installZigStyle("zls", version, ZlsIndexUrl, stripTopDir = false)
  of tkMsvc: installMsvc(version)
  of tkGcc: installGcc(version)
  of tkLlvmMingw: installLlvmMingw(version)
  of tkLua: installLua(version)
  else: raise newException(ValueError, "not a runtime: " & tool.name)

proc validatePackageName(package: string) =
  if package.len == 0 or package.isAbsolute or
      package.contains({' ', '\t', '\r', '\n'}):
    raise newException(ValueError, "invalid package name: " & package)
  for part in package.replace('\\', '/').split('/'):
    if part.len == 0 or part == "." or part == "..":
      raise newException(ValueError, "invalid package name: " & package)

proc environmentWithPath(directory: string): StringTableRef =
  when defined(windows):
    result = newStringTable(modeCaseInsensitive)
  else:
    result = newStringTable(modeCaseSensitive)
  for key, value in envPairs():
    result[key] = value
  result["PATH"] = directory & $PathSep & getEnv("PATH")

proc packageBins(packageJson: string): seq[(string, string)] =
  let package = parseFile(packageJson)
  if not package.hasKey("bin"):
    return
  let bin = package["bin"]
  case bin.kind
  of JString:
    let packageName = package["name"].getStr().rsplit('/', maxsplit = 1)[^1]
    result.add (packageName, bin.getStr())
  of JObject:
    for name, path in bin.pairs:
      if path.kind == JString:
        result.add (name, path.getStr())
  else:
    discard

proc validBinName(name: string): bool =
  name.len > 0 and not name.contains({' ', '\t', '\r', '\n', '/', '\\'})

proc safeScriptPath(path: string): bool =
  if path.len == 0 or path.isAbsolute:
    return false
  for part in path.replace('\\', '/').split('/'):
    if part == "..":
      return false
  true

proc createNpmBinLinks(staging, packageDir: string,
                       bins: openArray[(string, string)]) =
  let binDir = staging / "bin"
  createDir(binDir)
  for (name, script) in bins:
    if not name.validBinName or not script.safeScriptPath:
      raise newException(ValueError, "invalid npm executable mapping: " & name)
    let source = packageDir / script
    if not fileExists(source):
      raise newException(ValueError,
        "npm executable does not exist: " & source)
    createSymlink(relativePath(source, binDir), binDir / name)

proc installNpm(tool: ToolId, query, nodeVersion: string;
                force: bool): string =
  validatePackageName(tool.package)
  let nodeRoot = runtimeInstallDir(parseToolId("nodejs"), nodeVersion)
  if not runtimeComplete(parseToolId("nodejs"), nodeVersion):
    raise newException(ValueError, "Node.js is not installed: " & nodeVersion)

  let parentRoot = packageParentRoot(tool, nodeVersion)
  var lock = acquireInstallLock(parentRoot / ".install.lock")
  defer: lock.release()
  let existing = resolvePackageLocal(tool, query, nodeVersion)
  if existing.len > 0 and not force:
    return existing

  let staging = parentRoot / (".staging-" & fsComponent(query))
  removeDir(staging)
  createDir(staging)
  writeFile(staging / "package.json", "{\"private\":true}\n")

  let npmCli =
    when defined(windows):
      nodeRoot / "node_modules" / "npm" / "bin" / "npm-cli.js"
    else:
      nodeRoot / "lib" / "node_modules" / "npm" / "bin" / "npm-cli.js"
  let environment = environmentWithPath(nodeBin(nodeRoot))
  try:
    runChecked(nodeExecutable(nodeRoot), [
      npmCli,
      "install",
      "--no-audit",
      "--no-fund",
      "--no-save",
      "--prefix", staging,
      tool.package & "@" & query,
    ], environment)

    let packageDir = staging / "node_modules" / tool.package
    let packageJson = packageDir / "package.json"
    let document = parseFile(packageJson)
    let actual = document["version"].getStr()
    if actual.len == 0:
      raise newException(ValueError,
        tool.package & " did not report an installed version")
    let bins = packageBins(packageJson)
    if bins.len == 0:
      raise newException(ValueError, tool.package & " declares no executables")
    createNpmBinLinks(staging, packageDir, bins)
    removeFile(staging / "package.json")
    if fileExists(staging / "package-lock.json"):
      removeFile(staging / "package-lock.json")
    let destination = packageInstallDir(tool, actual, nodeVersion)
    var nameMap = beginNameMapTransaction()
    defer: nameMap.finishNameMapTransaction()
    nameMap.reservePackageCommands(tool, staging)
    createDir(parentDir(destination))
    if dirExists(destination):
      removeDir(staging)
    else:
      moveDir(staging, destination)
    nameMap.registerPackageCommands(tool)
    createPackageLink(tool, query, actual, nodeVersion)
    nameMap.commitNameMapTransaction()
    return actual
  except CatchableError:
    removeDir(staging)
    raise

proc goInstalledVersion(go, executable: string,
                        environment: StringTableRef): string =
  let output = captureCommand(go, ["version", "-m", executable], environment)
  for line in output.splitLines():
    let fields = line.splitWhitespace()
    if fields.len >= 3 and fields[0] == "mod":
      return fields[2]
  raise newException(ValueError,
    "could not determine Go module version for " & executable)

proc installGoPackage(tool: ToolId, query, goVersion: string;
                      force: bool): string =
  validatePackageName(tool.package)
  let goTool = parseToolId("golang")
  let goRoot = runtimeInstallDir(goTool, goVersion)
  if not runtimeComplete(goTool, goVersion):
    raise newException(ValueError, "Go is not installed: " & goVersion)

  let parentRoot = packageParentRoot(tool, goVersion)
  var lock = acquireInstallLock(parentRoot / ".install.lock")
  defer: lock.release()
  let existing = resolvePackageLocal(tool, query, goVersion)
  if existing.len > 0 and not force:
    return existing

  let staging = parentRoot / (".staging-" & fsComponent(query))
  removeDir(staging)
  createDir(staging / "bin")
  let environment = environmentWithPath(goRoot / "bin")
  environment["GOBIN"] = staging / "bin"
  try:
    let go = goExecutable(goRoot)
    runChecked(go, ["install", tool.package & "@" & query], environment)
    var actual: string
    for kind, path in walkDir(staging / "bin"):
      if kind != pcFile:
        continue
      let version = goInstalledVersion(go, path, environment)
      if actual.len == 0:
        actual = version
      elif actual != version:
        raise newException(ValueError,
          tool.package & " produced binaries from different module versions")
    if actual.len == 0:
      raise newException(ValueError, tool.package & " produced no executables")
    let destination = packageInstallDir(tool, actual, goVersion)
    var nameMap = beginNameMapTransaction()
    defer: nameMap.finishNameMapTransaction()
    nameMap.reservePackageCommands(tool, staging)
    createDir(parentDir(destination))
    if dirExists(destination):
      removeDir(staging)
    else:
      moveDir(staging, destination)
    nameMap.registerPackageCommands(tool)
    createPackageLink(tool, query, actual, goVersion)
    nameMap.commitNameMapTransaction()
    return actual
  except CatchableError:
    removeDir(staging)
    raise

proc rockDir(tree, shortVersion, rock: string): string =
  ## <tree>/lib/luarocks/rocks-<V>/<rock>; LuaRocks lowercases rock names.
  let rocks = tree / "lib" / "luarocks" / ("rocks-" & shortVersion)
  for kind, path in walkDir(rocks):
    if kind == pcDir and extractFilename(path).cmpIgnoreCase(rock) == 0:
      return path
  raise newException(ValueError, rock & " was not installed into " & rocks)

proc installLuabin(tool: ToolId, query, luaVersion: string; force: bool;
                   startDir: string): string =
  ## Installs a rock and its dependencies into a tree of their own. bin/
  ## links to the rock's scripts: LuaRocks' own wrappers hold the staging
  ## path, and dispatch runs the scripts with this tree's module paths.
  validatePackageName(tool.package)
  let luaTool = parseToolId("lua")
  let luaRoot = runtimeInstallDir(luaTool, luaVersion)
  if not runtimeComplete(luaTool, luaVersion):
    raise newException(ValueError, "Lua is not installed: " & luaVersion)

  let parentRoot = packageParentRoot(tool, luaVersion)
  var lock = acquireInstallLock(parentRoot / ".install.lock")
  defer: lock.release()
  let existing = resolvePackageLocal(tool, query, luaVersion)
  if existing.len > 0 and not force:
    return existing

  let staging = parentRoot / (".staging-" & fsComponent(query))
  removeDir(staging)
  createDir(staging)
  let luarocks = luarocksCommand(luaRoot)
  if not luarocks.isInstalled:
    raise newException(ValueError, "Lua " & luaVersion & " has no LuaRocks")
  let environment = currentEnvironment()
  execEnvironment(startDir).applyTo(environment)
  var arguments = luarocks.arguments & @["--tree", staging, "install",
    tool.package]
  if query != "latest":
    arguments.add query
  try:
    runChecked(luarocks.executable, arguments, environment)

    let rock = rockDir(staging, luaShortVersion(luaVersion), tool.package)
    var actual = ""
    for kind, path in walkDir(rock):
      if kind == pcDir:
        if actual.len > 0:
          raise newException(ValueError,
            tool.package & " was installed in more than one version")
        actual = extractFilename(path)
    if actual.len == 0:
      raise newException(ValueError,
        tool.package & " did not report an installed version")

    let binDir = staging / "bin"
    removeDir(binDir)
    createDir(binDir)
    for kind, script in walkDir(rock / actual / "bin"):
      if kind in {pcFile, pcLinkToFile}:
        createSymlink(relativePath(script, binDir),
          binDir / extractFilename(script))

    let destination = packageInstallDir(tool, actual, luaVersion)
    var nameMap = beginNameMapTransaction()
    defer: nameMap.finishNameMapTransaction()
    nameMap.reservePackageCommands(tool, staging)
    createDir(parentDir(destination))
    if dirExists(destination):
      removeDir(staging)
    else:
      moveDir(staging, destination)
    nameMap.registerPackageCommands(tool)
    createPackageLink(tool, query, actual, luaVersion)
    nameMap.commitNameMapTransaction()
    return actual
  except CatchableError:
    removeDir(staging)
    raise

proc report(action, tool, request, resolved: string) =
  var line = action & " " & tool & " " & request
  if resolved.len > 0 and resolved != request:
    line.add " -> " & resolved
  stdout.writeLine(line)

proc ensureRuntime(tool: ToolId, selector: string, options: InstallOptions,
                   resolved: var Table[string, string],
                   installed: var seq[InstalledSelection];
                   expose = true): string =
  let key = tool.name & "\x1f" & selector
  if resolved.hasKey(key):
    return resolved[key]

  if selector == "system" or selector.startsWith("path:"):
    if tool.isCToolchain:
      raise newException(ValueError,
        tool.name & " accepts only full toolchain names, got " & selector)
    result = selector
    resolved[key] = result
    installed.add InstalledSelection(tool: tool, request: selector, version: selector)
    return

  let local = resolveRuntimeLocal(tool, selector)
  # C toolchain names are full names, so an installed one is never refreshed.
  let refresh = options.mode == imUpdate and local.len > 0 and
    local != selector and not tool.isCToolchain
  if local.len > 0 and not refresh:
    report("skip", tool.name, selector, local)
    result = local
  else:
    result = resolveInstallVersion(tool, selector)
    let action = if local.len > 0: "update" else: "install"
    report(if options.dryRun: "would-" & action else: action,
      tool.name, selector, result)
    if not options.dryRun:
      discard installRuntime(tool, result)
      if not tool.isCToolchain:
        var selectorLock = acquireInstallLock(
          runtimeRoot(tool) / ".selectors.lock")
        defer: selectorLock.release()
        createRuntimeLinks(tool, selector, result)

  resolved[key] = result
  if expose:
    installed.add InstalledSelection(tool: tool, request: selector, version: result)

proc installAll*(selections: openArray[ToolSelection],
                 options = InstallOptions()): seq[InstalledSelection] =
  var resolved = initTable[string, string]()

  for selection in selections:
    let tool = parseToolId(selection.name)
    if not tool.isRuntime:
      continue
    for selector in selection.versions:
      discard ensureRuntime(tool, selector, options, resolved, result)

  for selection in selections:
    let tool = parseToolId(selection.name)
    if not tool.isPackage:
      continue
    if selection.versions.len != 1:
      raise newException(ValueError,
        selection.name & " requires exactly one <runtime>:<package> pair")
    let pair = parsePackagePair(selection.versions[0])
    if pair.runtime == "system" or pair.runtime.startsWith("path:"):
      raise newException(ValueError,
        selection.name & " requires a managed " & tool.parentTool & " runtime")

    let parent = tool.parentToolId
    let parentVersion = ensureRuntime(
      parent,
      pair.runtime,
      options,
      resolved,
      result,
      expose = false,
    )
    let local = resolvePackageLocal(tool, pair.package, parentVersion)
    let refresh = options.mode == imUpdate and local.len > 0 and local != pair.package
    var actual = local
    if local.len > 0 and not refresh:
      report("skip", tool.name, selection.versions[0], local)
    elif options.dryRun:
      report(
        if local.len > 0: "would-update" else: "would-install",
        tool.name,
        selection.versions[0],
        "",
      )
      actual = if local.len > 0: local else: pair.package
    else:
      report(
        if local.len > 0: "update" else: "install",
        tool.name,
        selection.versions[0],
        "",
      )
      case tool.kind
      of tkNpm:
        actual = installNpm(tool, pair.package, parentVersion, refresh)
      of tkGoPackage:
        actual = installGoPackage(tool, pair.package, parentVersion, refresh)
      of tkLuabin:
        actual = installLuabin(tool, pair.package, parentVersion, refresh,
          if options.startDir.len > 0: options.startDir else: getCurrentDir())
      else:
        discard

    if not options.dryRun:
      registerPackageCommands(tool)
    result.add InstalledSelection(
      tool: tool,
      request: pair.package,
      version: actual,
      parentVersion: parentVersion,
    )
