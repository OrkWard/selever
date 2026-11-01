## MSVC and the Windows SDK from the Visual Studio installer manifest.
##
## The VS 18 stable channel lists every MSVC toolset from 14.29 on as
## side-by-side packages, plus the current Windows SDKs. MSVC payloads are
## VSIX (zip) files; Windows SDK payloads are MSIs with external CABs.

import std/[algorithm, json, os, osproc, strutils, tables, uri]

import ./[catalog, environment, layout, lockfile, processutil, transport]

const
  VsChannelUrl = "https://aka.ms/vs/18/stable/channel"
  VsManifestItem = "Microsoft.VisualStudio.Manifests.VisualStudio"
  WindowsSdkRecord = "winsdk"
  SdkHeaderArchitectures = ["x64", "x86", "arm64", "arm"]
  ErrorInstallAlreadyRunning = 1618

type
  VsManifest = object
    packages: Table[string, seq[JsonNode]]

  VsPayload = object
    fileName: string
    url: string
    sha256: string

proc versionParts(version: string): seq[int] =
  for part in version.split('.'):
    if part.len == 0 or not part.allCharsInSet(Digits):
      return @[]
    result.add parseInt(part)

proc isNewer(candidate, current: string): bool =
  let left = versionParts(candidate)
  let right = versionParts(current)
  for index in 0 ..< max(left.len, right.len):
    let l = if index < left.len: left[index] else: 0
    let r = if index < right.len: right[index] else: 0
    if l != r:
      return l > r
  false

proc vsCacheDir(): string =
  cacheRoot() / "msvc"

proc loadVsManifest(): VsManifest =
  let channel = parseJson(fetchText(VsChannelUrl))
  var item: JsonNode
  for candidate in channel["channelItems"].items:
    if candidate["id"].getStr() == VsManifestItem:
      item = candidate["payloads"][0]
  if item.isNil:
    raise newException(ValueError,
      VsManifestItem & " is missing from the Visual Studio channel")
  # The served manifest does not match its declared hash, so the hash only
  # names the cache file.
  let path = vsCacheDir() / (item["sha256"].getStr().toLowerAscii() & ".vsman")
  download(item["url"].getStr(), path)
  for kind, old in walkDir(vsCacheDir()):
    if kind == pcFile and old.endsWith(".vsman") and
        old.extractFilename != path.extractFilename:
      removeFile(old)
  for package in parseFile(path)["packages"].items:
    result.packages.mgetOrPut(package["id"].getStr().toLowerAscii(), @[]).add package

proc lookup(manifest: VsManifest, id: string): JsonNode =
  ## Prefers the language-neutral package, then en-US.
  for package in manifest.packages.getOrDefault(id.toLowerAscii()):
    let language = package{"language"}.getStr()
    if language.len == 0:
      return package
    if language.cmpIgnoreCase("en-US") == 0:
      result = package

proc payloads(package: JsonNode): seq[VsPayload] =
  if not package.hasKey("payloads"):
    return
  for payload in package["payloads"].items:
    result.add VsPayload(
      fileName: payload["fileName"].getStr(),
      url: payload["url"].getStr(),
      sha256: payload["sha256"].getStr().toLowerAscii(),
    )

proc baseName(fileName: string): string =
  fileName.rsplit({'/', '\\'}, maxsplit = 1)[^1]

proc downloadPayloads(payloads: openArray[VsPayload]): seq[string] =
  ## Payload names repeat across packages, so the cache is keyed by hash.
  var downloads: seq[Download]
  for payload in payloads:
    result.add vsCacheDir() / "payloads" / payload.sha256 /
      payload.fileName.baseName
    downloads.add Download(url: payload.url, destination: result[^1],
      sha256: payload.sha256)
  verifiedDownloadAll(downloads)

# ---------------------------------------------------------------- MSVC

proc msvcComponent(manifest: VsManifest, toolset, host: string): string =
  ## The newest component (e.g. 14.44.17.14) of a toolset with a native compiler.
  const prefix = "microsoft.vc."
  let suffix = ".tools.host" & host & ".target" & host & ".base"
  for id in manifest.packages.keys:
    if not id.startsWith(prefix & toolset & ".") or not id.endsWith(suffix):
      continue
    let component = id[prefix.len ..< id.len - suffix.len]
    if versionParts(component).len > 0 and
        (result.len == 0 or component.isNewer(result)):
      result = component
  if result.len == 0:
    raise newException(ValueError,
      "MSVC toolset " & toolset & " is not in the Visual Studio manifest")

proc msvcPayloads(manifest: VsManifest, component, host: string): seq[VsPayload] =
  let prefix = "microsoft.vc." & component & "."
  var required = @[prefix & "crt.headers.base"]
  var optional = @[prefix & "asan.headers.base", prefix & "pgo.headers.base"]
  for target in MsvcTargets:
    let tools = "tools.host" & host & ".target" & target
    required.add [
      prefix & tools & ".base",
      prefix & "crt." & target & ".desktop.base",
      # Also holds msvcrt.lib, oldnames.lib and the other import libraries.
      prefix & "crt." & target & ".store.base",
    ]
    optional.add [
      prefix & tools & ".res.base",
      prefix & "premium." & tools & ".base",
      prefix & "pgo." & target & ".base",
      prefix & "asan." & target & ".base",
      prefix & "crt.redist." & target & ".base",
    ]
  var seen: seq[string]
  for id in required & optional:
    let package = manifest.lookup(id)
    if package.isNil:
      if id in required:
        raise newException(ValueError,
          "MSVC " & component & " is missing package " & id)
      continue
    for payload in package.payloads:
      if payload.sha256 notin seen:
        seen.add payload.sha256
        result.add payload

proc decodeNames(root: string) =
  ## VSIX entries are URL-encoded and tar keeps them literally. Renames
  ## deepest-first so parent paths stay valid.
  var paths: seq[string]
  for path in walkDirRec(root, yieldFilter = {pcFile, pcDir}):
    if '%' in path.extractFilename:
      paths.add path
  paths.sort(proc(a, b: string): int = cmp(b.len, a.len))
  for path in paths:
    let target = path.parentDir / decodeUrl(path.extractFilename, decodePlus = false)
    if dirExists(path): moveDir(path, target)
    else: moveFile(path, target)

proc msvcToolsDir*(root, host, target: string): string =
  ## VC/Tools/MSVC/<version> holding the compiler for host and target.
  for kind, path in walkDir(root / "VC" / "Tools" / "MSVC"):
    if kind == pcDir and
        fileExists(path / "bin" / ("Host" & host) / target / "cl.exe") and
        (result.len == 0 or path.extractFilename.isNewer(result.extractFilename)):
      result = path

proc msvcComplete*(root: string): bool =
  let native = nativeArchitecture()
  fileExists(root / WindowsSdkRecord) and
    msvcToolsDir(root, native, native).len > 0

# ---------------------------------------------------------------- Windows SDK

proc windowsSdkPackage(manifest: VsManifest, build: string): JsonNode =
  const prefix = "microsoft.visualstudio.component.windows1"
  for id, packages in manifest.packages:
    if not id.startsWith(prefix) or not id.endsWith("sdk." & build):
      continue
    if not packages[0].hasKey("dependencies"):
      continue
    for dependency in packages[0]["dependencies"].keys:
      let package = manifest.lookup(dependency)
      if not package.isNil and package.payloads.len > 0:
        return package
  raise newException(ValueError,
    "Windows SDK " & build & " is not in the Visual Studio manifest")

proc defaultWindowsSdk(manifest: VsManifest): string =
  ## The SDK the C++ Build Tools workload marks as recommended.
  const prefix = "microsoft.visualstudio.component.windows1"
  let workload = manifest.lookup("microsoft.visualstudio.workload.vctools")
  if not workload.isNil and workload.hasKey("dependencies"):
    for dependency, details in workload["dependencies"]:
      let id = dependency.toLowerAscii()
      if id.startsWith(prefix) and details.kind == JObject and
          details{"type"}.getStr() == "Recommended":
        return id.rsplit('.', maxsplit = 1)[^1]
  raise newException(ValueError,
    "the Visual Studio manifest recommends no Windows SDK")

proc windowsSdkInstallers(byName: Table[string, VsPayload]): seq[VsPayload] =
  var required = @[
    "Windows SDK for Windows Store Apps Tools-x86_en-us.msi",
    "Windows SDK for Windows Store Apps Headers-x86_en-us.msi",
    "Windows SDK for Windows Store Apps Headers OnecoreUap-x86_en-us.msi",
    "Windows SDK for Windows Store Apps Libs-x86_en-us.msi",
    "Universal CRT Headers Libraries and Sources-x86_en-us.msi",
  ]
  for target in MsvcTargets:
    required.add "Windows SDK Desktop Libs " & target & "-x86_en-us.msi"
  for name in required:
    let key = name.toLowerAscii()
    if key notin byName:
      raise newException(ValueError, "Windows SDK payload not found: " & name)
    result.add byName[key]
  # Desktop headers are split by architecture; take every one that exists.
  for architecture in SdkHeaderArchitectures:
    for name in [
      "Windows SDK Desktop Headers " & architecture & "-x86_en-us.msi",
      "Windows SDK OnecoreUap Headers " & architecture & "-x86_en-us.msi",
    ]:
      let key = name.toLowerAscii()
      if key in byName:
        result.add byName[key]

proc cabinetNames(msi: string): seq[string] =
  ## External CABs an MSI refers to: 32 hex digits + ".cab", stored as ASCII.
  let data = readFile(msi)
  var index = 0
  while true:
    let found = data.find(".cab", index)
    if found < 0:
      break
    index = found + 4
    if found >= 32 and data[found - 32 ..< found].allCharsInSet(HexDigits):
      let name = data[found - 32 ..< found].toLowerAscii() & ".cab"
      if name notin result:
        result.add name

proc extractMsi(msi, destination: string) =
  ## Unpacks an MSI with an administrative install: no elevation and nothing
  ## registered. External CABs must sit next to the MSI.
  let msiexec = getEnv("SystemRoot", "C:\\Windows") / "System32" / "msiexec.exe"
  let commandLine = quoteShell(msiexec) & " /a " & quoteShell(msi) &
    " /quiet /qn TARGETDIR=" & quoteShell(destination)
  for attempt in 1 .. 20:
    let process = startProcess(commandLine,
      options = {poEvalCommand, poParentStreams})
    let exitCode = process.waitForExit()
    process.close()
    if exitCode == QuitSuccess:
      return
    if exitCode != ErrorInstallAlreadyRunning:
      raise newException(OSError,
        "msiexec " & msi.extractFilename & " exited with status " & $exitCode)
    sleep(3000)
  raise newException(OSError,
    "msiexec " & msi.extractFilename & ": Windows Installer stayed busy")

proc windowsSdkVersion*(root: string): string =
  ## The Include/<version> directory holding the Windows headers.
  for kind, path in walkDir(root / "Windows Kits" / "10" / "Include"):
    if kind == pcDir and fileExists(path / "um" / "windows.h") and
        (result.len == 0 or path.extractFilename.isNewer(result.extractFilename)):
      result = path.extractFilename

proc installWindowsSdk(manifest: VsManifest, build: string) =
  let root = windowsSdkInstallDir(build)
  if windowsSdkVersion(root).len > 0:
    return
  var lock = acquireInstallLock(root & ".lock")
  defer: lock.release()
  if windowsSdkVersion(root).len > 0:
    return

  var byName: Table[string, VsPayload]
  for payload in windowsSdkPackage(manifest, build).payloads:
    byName[payload.fileName.baseName.toLowerAscii()] = payload
  let installers = downloadPayloads(windowsSdkInstallers(byName))
  var cabinetPayloads: seq[VsPayload]
  for installer in installers:
    for name in cabinetNames(installer):
      if name in byName and byName[name] notin cabinetPayloads:
        cabinetPayloads.add byName[name]
  let cabinets = downloadPayloads(cabinetPayloads)

  let staging = root & ".staging"
  let work = root & ".work"
  removeDir(staging)
  removeDir(work)
  createDir(staging)
  createDir(work)
  try:
    for file in installers & cabinets:
      let linked = work / file.extractFilename
      try:
        createHardlink(file, linked)
      except OSError:
        copyFile(file, linked)
    for installer in installers:
      let name = installer.extractFilename
      stdout.writeLine("extract winsdk " & build & " " & name)
      extractMsi(work / name, staging)
      # An administrative install also copies the MSI itself.
      removeFile(staging / name)
    if windowsSdkVersion(staging).len == 0:
      raise newException(ValueError,
        "Windows SDK " & build & " did not contain windows.h")
    removeDir(root)
    moveDir(staging, root)
  finally:
    removeDir(work)
    removeDir(staging)

# ---------------------------------------------------------------- install

proc installMsvc*(toolset: string): string =
  result = runtimeInstallDir(parseToolId("msvc"), toolset)
  if msvcComplete(result):
    return
  var lock = acquireInstallLock(result & ".lock")
  defer: lock.release()
  if msvcComplete(result):
    return

  let manifest = loadVsManifest()
  let host = nativeArchitecture()
  let component = msvcComponent(manifest, toolset, host)
  let sdkBuild = defaultWindowsSdk(manifest)
  installWindowsSdk(manifest, sdkBuild)

  let staging = result & ".staging"
  removeDir(staging)
  createDir(staging)
  try:
    for archive in downloadPayloads(msvcPayloads(manifest, component, host)):
      runChecked("tar", ["-xf", archive, "-C", staging])
    let contents = staging / "Contents"
    decodeNames(contents)
    # cl.exe starts vctip.exe to send telemetry; it outlives the build and
    # locks the toolset's files. cl works without it.
    for path in walkDirRec(contents / "VC" / "Tools" / "MSVC"):
      if path.extractFilename.cmpIgnoreCase("vctip.exe") == 0:
        removeFile(path)
    if msvcToolsDir(contents, host, host).len == 0:
      raise newException(ValueError,
        "MSVC " & component & " did not contain cl.exe")
    writeFile(contents / WindowsSdkRecord, sdkBuild & "\n")
    removeDir(result)
    createDir(parentDir(result))
    moveDir(contents, result)
  finally:
    removeDir(staging)

# ---------------------------------------------------------------- environment

proc msvcEnvironment*(root, target: string): EnvironmentChanges =
  ## What vcvarsall sets for a native host and the given target.
  let host = nativeArchitecture()
  let tools = msvcToolsDir(root, host, target)
  if tools.len == 0:
    raise newException(ValueError,
      "msvc in " & root & " has no compiler for target " & target)
  let sdkBuild = readFile(root / WindowsSdkRecord).strip()
  let sdkRoot = windowsSdkInstallDir(sdkBuild)
  let sdkVersion = windowsSdkVersion(sdkRoot)
  if sdkVersion.len == 0:
    raise newException(ValueError,
      "Windows SDK " & sdkBuild & " is not installed; run selever install")
  let kits = sdkRoot / "Windows Kits" / "10"
  let hostBin = tools / "bin" / ("Host" & host)
  let sdkInclude = kits / "Include" / sdkVersion
  let sdkLib = kits / "Lib" / sdkVersion
  let sdkBin = kits / "bin" / sdkVersion

  result.prepend("PATH", hostBin / target)
  if target != host:
    result.prepend("PATH", hostBin / host)
  # Host SDK tools (rc, mt), then the debug CRTs that /MDd programs load.
  result.prepend("PATH", sdkBin / host, sdkBin / target / "ucrt")
  for redist in walkDirs(root / "VC" / "Redist" / "MSVC" / "*"):
    for crt in walkDirs(redist / "debug_nonredist" / target / "*"):
      result.prepend("PATH", crt)
  result.prepend("INCLUDE", tools / "include")
  for part in ["ucrt", "shared", "um", "winrt", "cppwinrt"]:
    result.prepend("INCLUDE", sdkInclude / part)
  result.prepend("LIB", tools / "lib" / target, sdkLib / "ucrt" / target,
    sdkLib / "um" / target)
  result.prepend("LIBPATH", tools / "lib" / target)

  result.setVariable("VCINSTALLDIR", root / "VC" & "\\")
  result.setVariable("VCToolsInstallDir", tools & "\\")
  result.setVariable("VCToolsVersion", tools.extractFilename)
  result.setVariable("VSCMD_ARG_HOST_ARCH", host)
  result.setVariable("VSCMD_ARG_TGT_ARCH", target)
  result.setVariable("WindowsSdkDir", kits & "\\")
  result.setVariable("WindowsSDKVersion", sdkVersion & "\\")
  result.setVariable("WindowsSDKLibVersion", sdkVersion & "\\")
  result.setVariable("WindowsSdkBinPath", kits / "bin" & "\\")
  result.setVariable("WindowsSdkVerBinPath", sdkBin & "\\")
  result.setVariable("UniversalCRTSdkDir", kits & "\\")
  result.setVariable("UCRTVersion", sdkVersion)
  let compiler = hostBin / target / "cl.exe"
  result.setVariable("CC", compiler)
  result.setVariable("CXX", compiler)
