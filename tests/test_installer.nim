import std/[json, os, strutils, tempfiles, unittest]

import ../src/selever/[
  catalog,
  installer,
  layout,
  lockfile,
  namemap,
  resolver,
  shims,
  toolversions,
  transport,
]

proc executableName(name: string): string =
  when defined(windows): name & ".exe"
  else: name

proc writeExecutable(path: string; content = "") =
  writeFile(path, content)
  when not defined(windows):
    var permissions = getFilePermissions(path)
    permissions.incl fpUserExec
    setFilePermissions(path, permissions)

suite "installer":
  test "an existing runtime is reused without network":
    let root = createTempDir("selever-install-", "")
    defer: removeDir(root)
    putEnv("SELEVER_DATA_DIR", root / "data")
    defer: delEnv("SELEVER_DATA_DIR")

    let tool = parseToolId("nodejs")
    let install = runtimeInstallDir(tool, "0.0.0-test")
    let node = runtimeExecutable(tool, install)
    createDir(parentDir(node))
    writeExecutable(node)

    let installed = installAll([
      ToolSelection(name: "nodejs", versions: @["0.0.0-test"]),
    ])
    check installed.len == 1
    check installed[0].version == "0.0.0-test"

  test "an existing JDK is reused without network":
    let root = createTempDir("selever-java-install-", "")
    defer: removeDir(root)
    putEnv("SELEVER_DATA_DIR", root / "data")
    defer: delEnv("SELEVER_DATA_DIR")

    let tool = parseToolId("java")
    let version = "21.0.8+9"
    let install = runtimeInstallDir(tool, version)
    let suffix = when defined(windows): ".exe" else: ""
    createDir(install / "bin")
    writeExecutable(install / "bin" / ("java" & suffix))
    writeExecutable(install / "bin" / ("javac" & suffix))

    let installed = installAll([
      ToolSelection(name: "java", versions: @[version]),
    ])
    check installed.len == 1
    check installed[0].version == version

  test "Adoptium metadata has a stable concrete version":
    let featureRelease = parseJson("""
      {"version_data":{"major":25,"minor":0,"security":4,"patch":1,"build":1}}
    """)
    let latestRelease = parseJson("""
      {"version":{"major":21,"minor":0,"security":8,"build":9}}
    """)
    check javaVersion(featureRelease) == "25.0.4.1+1"
    check javaVersion(latestRelease) == "21.0.8+9"
    check resolveInstallVersion(
      parseToolId("java"), "temurin-21.0.8+9") == "21.0.8+9"

  test "a package requires exactly one explicit pair":
    expect ValueError:
      discard installAll([
        ToolSelection(name: "npm:typescript", versions: @["22:5.9.3", "20:5.8.3"]),
      ])
    expect ValueError:
      discard installAll([
        ToolSelection(name: "npm:typescript", versions: @["5.9.3"]),
      ])

  test "a package runtime remains private without a runtime selection":
    let root = createTempDir("selever-private-runtime-", "")
    defer: removeDir(root)
    putEnv("SELEVER_DATA_DIR", root / "data")
    defer: delEnv("SELEVER_DATA_DIR")

    let nodeTool = parseToolId("nodejs")
    let nodeRoot = runtimeInstallDir(nodeTool, "22.14.0")
    let node = runtimeExecutable(nodeTool, nodeRoot)
    createDir(parentDir(node))
    writeExecutable(node)

    let packageTool = parseToolId("npm:example")
    let packageRoot = packageInstallDir(packageTool, "1.0.0", "22.14.0")
    createDir(packageRoot / "bin")
    writeExecutable(packageRoot / "bin" / "example")
    createPackageLink(packageTool, "1.0.0", "1.0.0", "22.14.0")

    let installed = installAll([
      ToolSelection(name: packageTool.name, versions: @["22.14.0:1.0.0"]),
    ])
    check installed.len == 1
    check installed[0].tool == packageTool

  test "dry-run does not create a missing concrete runtime":
    let root = createTempDir("selever-dry-run-", "")
    defer: removeDir(root)
    putEnv("SELEVER_DATA_DIR", root / "data")
    defer: delEnv("SELEVER_DATA_DIR")

    let tool = parseToolId("nodejs")
    discard installAll(
      [ToolSelection(name: tool.name, versions: @["99.99.99"])],
      InstallOptions(dryRun: true),
    )
    check not dirExists(runtimeInstallDir(tool, "99.99.99"))

  test "package identity includes runtime and concrete package version":
    let tool = parseToolId("npm:typescript")
    check packageInstallDir(tool, "5.9.3", "20.0.0") !=
      packageInstallDir(tool, "5.9.3", "22.0.0")
    check packageInstallDir(parseToolId("npm:a-b"), "c", "20.0.0") !=
      packageInstallDir(parseToolId("npm:a"), "b-c", "20.0.0")

  test "runtime links resolve locally":
    let root = createTempDir("selever-links-", "")
    defer: removeDir(root)
    putEnv("SELEVER_DATA_DIR", root / "data")
    defer: delEnv("SELEVER_DATA_DIR")

    let tool = parseToolId("nodejs")
    let install = runtimeInstallDir(tool, "22.14.0")
    let node = runtimeExecutable(tool, install)
    createDir(parentDir(node))
    writeExecutable(node)
    createRuntimeLinks(tool, "lts/iron", "22.14.0")
    check resolveRuntimeLocal(tool, "22") == "22.14.0"
    check resolveRuntimeLocal(tool, "22.14") == "22.14.0"
    check resolveRuntimeLocal(tool, "lts/iron") == "22.14.0"

  when not defined(windows):
    test "runtime completeness requires execute permission":
      let root = createTempDir("selever-runtime-mode-", "")
      defer: removeDir(root)
      putEnv("SELEVER_DATA_DIR", root / "data")
      defer: delEnv("SELEVER_DATA_DIR")

      let tool = parseToolId("nodejs")
      let node = runtimeExecutable(tool, runtimeInstallDir(tool, "1.0.0"))
      createDir(parentDir(node))
      writeFile(node, "")
      check not runtimeComplete(tool, "1.0.0")
      writeExecutable(node)
      check runtimeComplete(tool, "1.0.0")

    test "name-map ignores non-executable package files":
      let root = createTempDir("selever-package-mode-", "")
      defer: removeDir(root)
      putEnv("SELEVER_DATA_DIR", root / "data")
      defer: delEnv("SELEVER_DATA_DIR")

      let tool = parseToolId("gopkg:example.invalid/modes")
      let bin = packageInstallDir(tool, "v1.0.0", "1.24.0") / "bin"
      createDir(bin)
      writeFile(bin / "disabled", "")
      writeExecutable(bin / "enabled")
      registerPackageCommands(tool)

      check lookupName("disabled").command.len == 0
      check lookupName("enabled").tool == tool.name

  test "name-map rejects Java command ownership":
    let root = createTempDir("selever-java-map-", "")
    defer: removeDir(root)
    putEnv("SELEVER_DATA_DIR", root / "data")
    defer: delEnv("SELEVER_DATA_DIR")

    let tool = parseToolId("npm:example")
    let bin = packageInstallDir(tool, "1.0.0", "22.0.0") / "bin"
    createDir(bin)
    writeExecutable(bin / "javac")
    expect ValueError:
      registerPackageCommands(tool)

  test "name-map reservation blocks takeover and permits recovery":
    let root = createTempDir("selever-map-reservation-", "")
    defer: removeDir(root)
    putEnv("SELEVER_DATA_DIR", root / "data")
    defer: delEnv("SELEVER_DATA_DIR")

    let first = parseToolId("gopkg:example.invalid/one")
    let firstStaging = root / "first-staging"
    createDir(firstStaging / "bin")
    writeExecutable(firstStaging / "bin" / "shared")

    var rolledBack = beginNameMapTransaction()
    rolledBack.reservePackageCommands(first, firstStaging)
    rolledBack.finishNameMapTransaction()
    check lookupName("shared").command.len == 0

    var reserved = beginNameMapTransaction()
    reserved.reservePackageCommands(first, firstStaging)
    reserved.commitNameMapTransaction()
    reserved.finishNameMapTransaction()
    check lookupName("shared").tool == first.name

    let second = parseToolId("gopkg:example.invalid/two")
    let secondStaging = root / "second-staging"
    createDir(secondStaging / "bin")
    writeExecutable(secondStaging / "bin" / "shared")
    var conflict = beginNameMapTransaction()
    expect ValueError:
      conflict.reservePackageCommands(second, secondStaging)
    conflict.finishNameMapTransaction()

    let installedBin =
      packageInstallDir(first, "v1.0.0", "1.24.0") / "bin"
    createDir(installedBin)
    writeExecutable(installedBin / "shared")
    var recovery = beginNameMapTransaction()
    recovery.reservePackageCommands(first, firstStaging)
    recovery.registerPackageCommands(first)
    recovery.commitNameMapTransaction()
    recovery.finishNameMapTransaction()
    check lookupName("shared").tool == first.name

  test "name-map rejects command ownership conflicts":
    let root = createTempDir("selever-map-", "")
    defer: removeDir(root)
    putEnv("SELEVER_DATA_DIR", root / "data")
    defer: delEnv("SELEVER_DATA_DIR")

    let first = parseToolId("gopkg:example.invalid/one")
    let firstBin = packageInstallDir(first, "v1.0.0", "1.24.0") / "bin"
    createDir(firstBin)
    writeExecutable(firstBin / "shared")
    registerPackageCommands(first)
    check lookupName("shared").tool == first.name

    let second = parseToolId("gopkg:example.invalid/two")
    let secondBin = packageInstallDir(second, "v1.0.0", "1.24.0") / "bin"
    createDir(secondBin)
    writeExecutable(secondBin / "shared")
    expect ValueError:
      registerPackageCommands(second)

  test "name-map unions commands from every installed version":
    let root = createTempDir("selever-map-", "")
    defer: removeDir(root)
    putEnv("SELEVER_DATA_DIR", root / "data")
    defer: delEnv("SELEVER_DATA_DIR")

    let tool = parseToolId("npm:example")
    let oldBin = packageInstallDir(tool, "1.0.0", "20.0.0") / "bin"
    let newBin = packageInstallDir(tool, "2.0.0", "22.0.0") / "bin"
    createDir(oldBin)
    createDir(newBin)
    writeExecutable(oldBin / "old-command")
    writeExecutable(newBin / "new-command")
    registerPackageCommands(tool)
    check lookupName("old-command").tool == tool.name
    check lookupName("new-command").tool == tool.name

  test "install locks are exclusive and reusable":
    let root = createTempDir("selever-lock-", "")
    defer: removeDir(root)
    let path = root / "tool.lock"
    var first = acquireInstallLock(path)
    expect IOError:
      discard acquireInstallLock(path, timeoutMs = 50)
    first.release()
    var second = acquireInstallLock(path, timeoutMs = 50)
    second.release()

  test "SHA-256 verification rejects mismatches":
    let root = createTempDir("selever-hash-", "")
    defer: removeDir(root)
    let path = root / "payload"
    writeFile(path, "abc")
    let expected =
      "ba7816bf8f01cfea414140de5dae2223" &
      "b00361a396177a9cb410ff61f20015ad"
    check sha256File(path) == expected
    verifySha256(path, expected)
    expect ValueError:
      verifySha256(path, repeat('0', 64))

  test "sync creates Java shims from installed JDK commands":
    let root = createTempDir("selever-java-shims-", "")
    defer: removeDir(root)
    putEnv("SELEVER_DATA_DIR", root / "data")
    putEnv("SELEVER_CONFIG_DIR", root / "config")
    putEnv("SELEVER_SHIMS_DIR", root / "shims")
    defer:
      delEnv("SELEVER_DATA_DIR")
      delEnv("SELEVER_CONFIG_DIR")
      delEnv("SELEVER_SHIMS_DIR")

    let tool = parseToolId("java")
    let version = "21.0.8+9"
    let install = runtimeInstallDir(tool, version)
    let suffix = when defined(windows): ".exe" else: ""
    createDir(install / "bin")
    writeExecutable(install / "bin" / ("java" & suffix))
    writeExecutable(install / "bin" / ("javac" & suffix))

    let dispatcher = root / executableName("selever")
    writeFile(dispatcher, "dispatcher")
    syncShims([
      InstalledSelection(tool: tool, request: "21", version: version),
    ], dispatcher)
    for command in ["java", "javac"]:
      let shim = shimDir() / executableName(command)
      check fileExists(shim)
      check sameFile(dispatcher, shim)

  when defined(windows):
    test "Windows shims target the stable Scoop current path":
      let root = createTempDir("selever-current-shim-", "")
      defer: removeDir(root)
      putEnv("SELEVER_DATA_DIR", root / "data")
      putEnv("SELEVER_CONFIG_DIR", root / "config")
      putEnv("SELEVER_SHIMS_DIR", root / "shims")
      defer:
        delEnv("SELEVER_DATA_DIR")
        delEnv("SELEVER_CONFIG_DIR")
        delEnv("SELEVER_SHIMS_DIR")

      let tool = parseToolId("nodejs")
      let version = "0.0.0-test"
      let node = runtimeExecutable(tool, runtimeInstallDir(tool, version))
      createDir(parentDir(node))
      writeExecutable(node)

      let appRoot = root / "apps" / "selever"
      let dispatcher = appRoot / "2.2.1" / "selever.exe"
      let current = appRoot / "current" / "selever.exe"
      createDir(parentDir(dispatcher))
      createDir(parentDir(current))
      writeFile(dispatcher, "versioned")
      writeFile(current, "current")

      syncShims([
        InstalledSelection(tool: tool, request: version, version: version),
      ], dispatcher)
      let shim = shimDir() / "node.exe"
      check symlinkExists(shim)
      check sameFile(current, shim)
      check not sameFile(dispatcher, shim)

  test "sync creates a native dispatcher shim":
    let root = createTempDir("selever-shims-", "")
    defer: removeDir(root)
    putEnv("SELEVER_DATA_DIR", root / "data")
    putEnv("SELEVER_CONFIG_DIR", root / "config")
    putEnv("SELEVER_SHIMS_DIR", root / "shims")
    defer:
      delEnv("SELEVER_DATA_DIR")
      delEnv("SELEVER_CONFIG_DIR")
      delEnv("SELEVER_SHIMS_DIR")

    let tool = parseToolId("nodejs")
    let install = runtimeInstallDir(tool, "0.0.0-test")
    let node = runtimeExecutable(tool, install)
    createDir(parentDir(node))
    writeExecutable(node)

    let dispatcher = root / executableName("selever")
    writeFile(dispatcher, "dispatcher")
    syncShims([
      InstalledSelection(
        tool: tool,
        request: "0.0.0-test",
        version: "0.0.0-test",
      ),
    ], dispatcher)
    let shim = shimDir() / executableName("node")
    check fileExists(shim)
    check symlinkExists(shim)
    check sameFile(dispatcher, shim)
