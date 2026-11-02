import std/[strutils, unittest]

import ../src/selever/cli

suite "cli":
  test "bare invocation displays help successfully":
    check runCli(@[]) == QuitSuccess

  test "help displays successfully":
    check runCli(@["--help"]) == QuitSuccess

  test "help documents supported tools and selectors":
    let help = usage()
    for tool in ["nodejs", "golang", "java", "zig", "zls",
                 "npm:<package>", "gopkg:<module>"]:
      check help.contains(tool)
    check help.contains("system and path:<directory>")
    check help.contains("<node-selector>:<package-selector>")

  test "exec requires its delimiter and a command":
    check runCli(@["exec"]) == QuitUsage
    check runCli(@["exec", "java"]) == QuitUsage
    check runCli(@["exec", "--"]) == QuitUsage

  test "unknown arguments are usage errors":
    check runCli(@["--unknown"]) == QuitUsage
