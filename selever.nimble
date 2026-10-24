version = "2.1.0"
author = "OrkWard"
description = "Fast project tool version dispatcher"
license = "MIT"

srcDir = "src"
bin = @["selever"]

requires "nim >= 2.2.0"

task test, "Run the Nim test suite":
  exec "nim c -r --hints:off tests/test_toolversions.nim"
  exec "nim c -r --hints:off tests/test_dispatch.nim"
  exec "nim c -r --hints:off tests/test_installer.nim"
  exec "nim c -r --hints:off tests/test_cli.nim"
