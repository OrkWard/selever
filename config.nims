switch("mm", "arc")
switch("define", "useMalloc")
switch("styleCheck", "error")
switch("warningAsError", "UnusedImport")
# begin Nimble config (version 2)
--noNimblePath
when withDir(thisDir(), system.fileExists("nimble.paths")):
  include "nimble.paths"
# end Nimble config
