import std/[os, strtabs, tables, tempfiles, unittest]

import ../src/selever/[
  catalog,
  environment,
  execenv,
  installer,
  layout,
  resolver,
  toolversions,
]

proc executableName(name: string): string =
  when defined(windows): name & ".exe"
  else: name

proc writeExecutable(path: string) =
  createDir(parentDir(path))
  writeFile(path, "")
  when not defined(windows):
    var permissions = getFilePermissions(path)
    permissions.incl fpUserExec
    setFilePermissions(path, permissions)

proc configureRoots(root: string) =
  putEnv("SELEVER_DATA_DIR", root / "data")
  putEnv("SELEVER_CONFIG_DIR", root / "config")

proc clearRoots() =
  delEnv("SELEVER_DATA_DIR")
  delEnv("SELEVER_CONFIG_DIR")

proc createMsvc(toolset: string; targets: openArray[string]): string =
  ## A fake MSVC install with Windows SDK 26100.
  result = runtimeInstallDir(parseToolId("msvc"), toolset)
  let host = nativeArchitecture()
  for target in targets:
    writeExecutable(result / "VC" / "Tools" / "MSVC" / "14.44.35207" / "bin" /
      ("Host" & host) / target / "cl.exe")
  writeFile(result / "winsdk", "26100\n")
  let headers = windowsSdkInstallDir("26100") / "Windows Kits" / "10" /
    "Include" / "10.0.26100.0" / "um"
  createDir(headers)
  writeFile(headers / "windows.h", "")

suite "exec environment":
  test "msvc selectors carry an optional target":
    let native = parseMsvcSelector("14.44")
    check native.toolset == "14.44"
    check native.target == nativeArchitecture()
    check parseMsvcSelector("14.44-arm64").target == "arm64"
    for invalid in ["latest", "14", "14.44.35207", "14.44-arm", "14.44-"]:
      expect ValueError:
        discard parseMsvcSelector(invalid)

  test "the JDK is exposed through JAVA_HOME and PATH":
    let root = createTempDir("selever-exec-java-", "")
    defer: removeDir(root)
    defer: clearRoots()
    configureRoots(root)
    writeFile(root / ".tool-versions", "java 21\n")

    let tool = parseToolId("java")
    let install = runtimeInstallDir(tool, "21.0.8+9")
    writeExecutable(install / "bin" / executableName("java"))
    writeExecutable(install / "bin" / executableName("javac"))
    createRuntimeLinks(tool, "21", "21.0.8+9")

    let changes = execEnvironment(root)
    check changes.variables["JAVA_HOME"] == install
    check changes.prepends["PATH"] == @[install / "bin"]

  test "the first toolchain wins PATH, CC and CXX":
    let root = createTempDir("selever-exec-mingw-", "")
    defer: removeDir(root)
    defer: clearRoots()
    configureRoots(root)
    writeFile(root / ".tool-versions",
      "llvm-mingw 20261020\ngcc 16.2.0posix-14.0.0-ucrt-r1\n")

    let gcc = runtimeInstallDir(parseToolId("gcc"), "16.2.0posix-14.0.0-ucrt-r1")
    writeExecutable(gcc / "bin" / executableName("gcc"))
    let clang = runtimeInstallDir(parseToolId("llvm-mingw"), "20261020")
    writeExecutable(clang / "bin" / executableName("clang"))

    let changes = execEnvironment(root)
    check changes.prepends["PATH"] == @[clang / "bin", gcc / "bin"]
    check changes.variables["CC"] == clang / "bin" / executableName("clang")
    check changes.variables["CXX"] == clang / "bin" / executableName("clang++")

  test "msvc targets the selected architecture":
    let root = createTempDir("selever-exec-msvc-", "")
    defer: removeDir(root)
    defer: clearRoots()
    configureRoots(root)
    writeFile(root / ".tool-versions", "msvc 14.44-arm64\n")

    let install = createMsvc("14.44", [nativeArchitecture(), "arm64"])
    check resolveRuntimeLocal(parseToolId("msvc"), "14.44-arm64") == "14.44"

    let changes = execEnvironment(root)
    let tools = install / "VC" / "Tools" / "MSVC" / "14.44.35207"
    let hostBin = tools / "bin" / ("Host" & nativeArchitecture())
    check changes.variables["CC"] == hostBin / "arm64" / "cl.exe"
    check changes.variables["VSCMD_ARG_TGT_ARCH"] == "arm64"
    check changes.variables["VCToolsVersion"] == "14.44.35207"
    check changes.prepends["PATH"][0] == hostBin / "arm64"
    check changes.prepends["LIB"][0] == tools / "lib" / "arm64"

  test "a missing msvc target is an error":
    let root = createTempDir("selever-exec-msvc-", "")
    defer: removeDir(root)
    defer: clearRoots()
    configureRoots(root)
    writeFile(root / ".tool-versions", "msvc 14.44-x86\n")
    discard createMsvc("14.44", [nativeArchitecture()])
    expect ValueError:
      discard execEnvironment(root)

  test "C toolchains reject system and path selectors":
    expect ValueError:
      discard installAll([
        ToolSelection(name: "gcc", versions: @["system"]),
      ])

  test "changes prepend lists and override variables":
    var changes: EnvironmentChanges
    changes.prepend("PATH", "first", "second")
    changes.setVariable("CC", "cl")
    let environment = newStringTable({"PATH": "old", "CC": "gcc"})
    changes.applyTo(environment)
    check environment["PATH"] ==
      "first" & $PathSep & "second" & $PathSep & "old"
    check environment["CC"] == "cl"

  test "commands are found on the new PATH":
    let root = createTempDir("selever-exec-find-", "")
    defer: removeDir(root)
    let executable = root / executableName("tool")
    writeExecutable(executable)
    check findCommand("tool", root) == executable
    check findCommand("missing", root) == "missing"
