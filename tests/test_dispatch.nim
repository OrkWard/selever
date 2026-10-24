import std/[os, tempfiles, unittest]

import ../src/selever/[catalog, dispatch, layout, resolver]

when not defined(windows):
  import ../src/selever/namemap

proc executableName(name: string): string =
  when defined(windows): name & ".exe"
  else: name

proc writeExecutable(path: string; content = "") =
  writeFile(path, content)
  when not defined(windows):
    var permissions = getFilePermissions(path)
    permissions.incl fpUserExec
    setFilePermissions(path, permissions)

proc configureRoots(root: string) =
  putEnv("SELEVER_DATA_DIR", root / "data")
  putEnv("SELEVER_CONFIG_DIR", root / "config")
  putEnv("SELEVER_SHIMS_DIR", root / "shims")

proc clearRoots() =
  delEnv("SELEVER_DATA_DIR")
  delEnv("SELEVER_CONFIG_DIR")
  delEnv("SELEVER_SHIMS_DIR")

proc createRuntime(tool: ToolId, version: string): string =
  result = runtimeInstallDir(tool, version)
  let executable = runtimeExecutable(tool, result)
  createDir(parentDir(executable))
  writeExecutable(executable)

suite "dispatcher":
  test "missing installation is an error":
    let root = createTempDir("selever-dispatch-", "")
    defer: removeDir(root)
    defer: clearRoots()
    configureRoots(root)
    writeFile(root / ".tool-versions", "nodejs 0.0.0-test\n")

    expect ValueError:
      discard resolveCommand("node", root)

  test "resolves a runtime selector locally":
    let root = createTempDir("selever-dispatch-", "")
    defer: removeDir(root)
    defer: clearRoots()
    configureRoots(root)
    writeFile(root / ".tool-versions", "nodejs 22\n")

    let tool = parseToolId("nodejs")
    let install = createRuntime(tool, "22.14.0")
    createRuntimeLinks(tool, "22", "22.14.0")

    let resolved = resolveCommand("node", root)
    check resolved.executable == runtimeExecutable(tool, install)
    check resolved.version == "22"

  test "pins Node for an npm package command":
    let root = createTempDir("selever-dispatch-", "")
    defer: removeDir(root)
    defer: clearRoots()
    configureRoots(root)
    writeFile(root / ".tool-versions",
      "npm:typescript 22:5.9.3\n")

    let nodeTool = parseToolId("nodejs")
    let nodeRoot = createRuntime(nodeTool, "22.14.0")
    createRuntimeLinks(nodeTool, "22", "22.14.0")

    let tool = parseToolId("npm:typescript")
    let packageRoot = packageInstallDir(tool, "5.9.3", "22.14.0")
    let script = packageRoot / "node_modules" / "typescript" / "bin" / "tsc"
    createDir(parentDir(script))
    writeExecutable(script, "#!/usr/bin/env node\n")
    createDir(packageRoot / "bin")
    createSymlink(relativePath(script, packageRoot / "bin"),
      packageRoot / "bin" / "tsc")
    createPackageLink(tool, "5.9.3", "5.9.3", "22.14.0")
    createDir(dataRoot())
    writeFile(nameMapPath(),
      "tsc npm:typescript bin/tsc\n")

    let resolved = resolveCommand("tsc", root)
    check resolved.executable == runtimeExecutable(nodeTool, nodeRoot)
    check resolved.arguments == @[packageRoot / "bin" / "tsc"]

  test "resolves a Go package executable through name-map":
    let root = createTempDir("selever-dispatch-", "")
    defer: removeDir(root)
    defer: clearRoots()
    configureRoots(root)
    writeFile(root / ".tool-versions",
      "gopkg:example.invalid/gopls 1.24:v1.0.0\n")

    let goTool = parseToolId("golang")
    discard createRuntime(goTool, "1.24.1")
    createRuntimeLinks(goTool, "1.24", "1.24.1")

    let tool = parseToolId("gopkg:example.invalid/gopls")
    let packageRoot = packageInstallDir(tool, "v1.0.0", "1.24.1")
    let executable = packageRoot / "bin" / executableName("gopls")
    createDir(parentDir(executable))
    writeExecutable(executable)
    createPackageLink(tool, "v1.0.0", "v1.0.0", "1.24.1")
    createDir(dataRoot())
    writeFile(nameMapPath(),
      "gopls gopkg:example.invalid/gopls bin/gopls\n")

    let resolved = resolveCommand("gopls", root)
    check resolved.executable == executable

  when not defined(windows):
    test "preserves dots in Unix command names":
      let root = createTempDir("selever-dotted-command-", "")
      defer: removeDir(root)
      defer: clearRoots()
      configureRoots(root)
      writeFile(root / ".tool-versions",
        "gopkg:example.invalid/dotted 1.24:v1.0.0\n")

      let goTool = parseToolId("golang")
      discard createRuntime(goTool, "1.24.1")
      createRuntimeLinks(goTool, "1.24", "1.24.1")

      let tool = parseToolId("gopkg:example.invalid/dotted")
      let packageRoot = packageInstallDir(tool, "v1.0.0", "1.24.1")
      let executable = packageRoot / "bin" / "tool.js"
      createDir(parentDir(executable))
      writeExecutable(executable)
      createPackageLink(tool, "v1.0.0", "v1.0.0", "1.24.1")
      registerPackageCommands(tool)

      check invocationName("/tmp/tool.js") == "tool.js"
      check lookupName("tool.js").locator == "bin/tool.js"
      check resolveCommand("tool.js", root).executable == executable

    test "system selection skips non-executable PATH entries":
      let root = createTempDir("selever-system-path-", "")
      defer: removeDir(root)
      defer: clearRoots()
      configureRoots(root)
      let first = root / "first"
      let second = root / "second"
      createDir(first)
      createDir(second)
      writeFile(first / "node", "")
      writeExecutable(second / "node")
      let oldPath = getEnv("PATH")
      defer: putEnv("PATH", oldPath)
      putEnv("PATH", first & $PathSep & second)
      writeFile(root / ".tool-versions", "nodejs system\n")

      check resolveCommand("node", root).executable == second / "node"

  test "path selection does not claim another tool command":
    let root = createTempDir("selever-dispatch-", "")
    defer: removeDir(root)
    defer: clearRoots()
    configureRoots(root)
    let custom = root / "custom"
    createDir(custom)
    writeFile(custom / executableName("go"), "")
    writeFile(root / ".tool-versions", "nodejs path:" & custom & "\n")

    expect ValueError:
      discard resolveCommand("go", root)
