import std/[os, strutils, tempfiles, unittest]

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
    writeFile(node, "")

    let installed = installAll([
      ToolSelection(name: "nodejs", versions: @["0.0.0-test"]),
    ])
    check installed.len == 1
    check installed[0].version == "0.0.0-test"

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
    writeFile(node, "")

    let packageTool = parseToolId("npm:example")
    let packageRoot = packageInstallDir(packageTool, "1.0.0", "22.14.0")
    createDir(packageRoot / "bin")
    writeFile(packageRoot / "bin" / "example", "")
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
    writeFile(node, "")
    createRuntimeLinks(tool, "lts/iron", "22.14.0")
    check resolveRuntimeLocal(tool, "22") == "22.14.0"
    check resolveRuntimeLocal(tool, "22.14") == "22.14.0"
    check resolveRuntimeLocal(tool, "lts/iron") == "22.14.0"

  test "name-map rejects command ownership conflicts":
    let root = createTempDir("selever-map-", "")
    defer: removeDir(root)
    putEnv("SELEVER_DATA_DIR", root / "data")
    defer: delEnv("SELEVER_DATA_DIR")

    let first = parseToolId("gopkg:example.invalid/one")
    let firstBin = packageInstallDir(first, "v1.0.0", "1.24.0") / "bin"
    createDir(firstBin)
    writeFile(firstBin / "shared", "")
    registerPackageCommands(first)
    check lookupName("shared").tool == first.name

    let second = parseToolId("gopkg:example.invalid/two")
    let secondBin = packageInstallDir(second, "v1.0.0", "1.24.0") / "bin"
    createDir(secondBin)
    writeFile(secondBin / "shared", "")
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
    writeFile(oldBin / "old-command", "")
    writeFile(newBin / "new-command", "")
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
    writeFile(node, "")

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
    check sameFile(dispatcher, shim)
