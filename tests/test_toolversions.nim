import std/[os, tempfiles, unittest]

import ../src/selever/[catalog, toolversions]

suite "tool versions":
  test "parses comments and runtime fallbacks":
    let selections = parseToolVersionsContent(
      "nodejs 22 20 # fallback\n# comment\ngolang 1.24\n")
    check selections.len == 2
    check selections[0].name == "nodejs"
    check selections[0].versions == @["22", "20"]
    check selections[1].name == "golang"

  test "package selections use one explicit runtime pair":
    let pair = parsePackagePair("lts/iron:latest")
    check pair.runtime == "lts/iron"
    check pair.package == "latest"
    expect ValueError:
      discard parsePackagePair("latest")

  test "global config provides lowest-priority per-tool fallbacks":
    let root = createTempDir("selever-tv-", "")
    defer: removeDir(root)
    let child = root / "project" / "subdir"
    let config = root / "config"
    createDir(root / "project")
    createDir(child)
    createDir(config)
    writeFile(root / ".tool-versions", "nodejs 20\ngolang 1.24\n")
    writeFile(root / "project" / ".tool-versions", "nodejs 22\n")
    writeFile(config / "tool-versions",
      "nodejs 18\ngolang 1.23\nzig 0.13.0\n")

    let documents = discoveredToolVersions(child, config)
    check documents[^1].path == config / "tool-versions"
    let selections = visibleSelections(child, config)
    let nodeIndex = selections.findSelection("nodejs")
    let goIndex = selections.findSelection("golang")
    let zigIndex = selections.findSelection("zig")
    check nodeIndex >= 0
    check goIndex >= 0
    check zigIndex >= 0
    check selections[nodeIndex].versions == @["22"]
    check selections[goIndex].versions == @["1.24"]
    check selections[zigIndex].versions == @["0.13.0"]
    check selections[zigIndex].source == config / "tool-versions"

  test "SELEVER_CONFIG_DIR selects the global file":
    let root = createTempDir("selever-tv-global-", "")
    defer: removeDir(root)
    let workspace = root / "workspace"
    let config = root / "config"
    createDir(workspace)
    createDir(config)
    writeFile(config / "tool-versions", "zls 0.14.0\n")
    putEnv("SELEVER_CONFIG_DIR", config)
    defer: delEnv("SELEVER_CONFIG_DIR")

    let selection = resolveSelection("zls", workspace)
    check selection.versions == @["0.14.0"]
    check selection.source == config / "tool-versions"

  test "nearest duplicate wins":
    let root = createTempDir("selever-tv-", "")
    defer: removeDir(root)
    let child = root / "child"
    createDir(child)
    writeFile(root / ".tool-versions", "zig 0.13.0\n")
    writeFile(child / ".tool-versions", "zig 0.14.0\nzig 0.15.0\n")

    let selection = resolveSelection("zig", child, root / "config")
    check selection.versions == @["0.14.0"]
