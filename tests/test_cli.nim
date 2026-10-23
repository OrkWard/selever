import std/unittest

import ../src/selever/cli

suite "cli":
  test "bare invocation displays help successfully":
    check runCli(@[]) == 0

  test "help displays successfully":
    check runCli(@["--help"]) == 0

  test "unknown arguments are usage errors":
    check runCli(@["--unknown"]) == 2
