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

  test "resolves each tool independently through parents":
    let root = createTempDir("selever-tv-", "")
    defer: removeDir(root)
    let child = root / "project" / "subdir"
    createDir(root / "project")
    createDir(child)
    writeFile(root / ".tool-versions", "nodejs 20\ngolang 1.24\n")
    writeFile(root / "project" / ".tool-versions", "nodejs 22\n")

    let previous = getCurrentDir()
    setCurrentDir(child)
    defer: setCurrentDir(previous)
    let documents = discoveredToolVersions(child, root)
    check documents.len == 2
    let selections = visibleSelections(child, root)
    check selections.len == 2
    check selections[0].name == "nodejs"
    check selections[0].versions == @["22"]
    check selections[1].name == "golang"

  test "nearest duplicate wins":
    let root = createTempDir("selever-tv-", "")
    defer: removeDir(root)
    let child = root / "child"
    createDir(child)
    writeFile(root / ".tool-versions", "zig 0.13.0\n")
    writeFile(child / ".tool-versions", "zig 0.14.0\nzig 0.15.0\n")

    let selection = resolveSelection("zig", child, root)
    check selection.versions == @["0.14.0"]
