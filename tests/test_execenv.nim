import std/[os, strtabs, tables, tempfiles, unittest]

import ../src/selever/[
  catalog,
  environment,
  execenv,
  layout,
  resolver,
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

suite "exec environment":
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
