import std/[os, strutils, tables]

import ./layout

const
  ToolVersionsFile* = ".tool-versions"
  GlobalToolVersionsFile* = "tool-versions"

type
  ToolSelection* = object
    name*: string
    versions*: seq[string]
    directory*: string
    source*: string

  ToolVersionsDocument* = object
    path*: string
    selections*: seq[ToolSelection]

proc parseToolVersionsContent*(content: string; directory = "";
                               source = ToolVersionsFile): seq[ToolSelection] =
  for line in content.splitLines():
    let body = line.split('#', maxsplit = 1)[0]
    let fields = body.splitWhitespace()
    if fields.len < 2:
      continue
    result.add ToolSelection(
      name: fields[0],
      versions: fields[1 .. ^1],
      directory: directory,
      source: source,
    )

proc readToolVersions*(path: string): seq[ToolSelection] =
  parseToolVersionsContent(readFile(path), parentDir(path), path)

proc discoveredToolVersions*(startDir: string,
                             configDir = ""): seq[ToolVersionsDocument] =
  var directory = normalizedPath(absolutePath(startDir))
  while true:
    let path = directory / ToolVersionsFile
    if fileExists(path):
      result.add ToolVersionsDocument(path: path, selections: readToolVersions(path))
    let parent = parentDir(directory)
    if parent.len == 0 or cmpPaths(parent, directory) == 0:
      break
    directory = parent

  let globalRoot =
    if configDir.len > 0: normalizedPath(absolutePath(configDir))
    else: configRoot()
  let globalPath = globalRoot / GlobalToolVersionsFile
  if fileExists(globalPath):
    result.add ToolVersionsDocument(
      path: globalPath,
      selections: readToolVersions(globalPath),
    )

proc visibleSelections*(startDir: string,
                        configDir = ""): seq[ToolSelection] =
  var seen = initTable[string, bool]()
  for document in discoveredToolVersions(startDir, configDir):
    for selection in document.selections:
      if not seen.hasKey(selection.name):
        seen[selection.name] = true
        result.add selection

proc findSelection*(selections: openArray[ToolSelection],
                    name: string): int =
  for index, selection in selections:
    if selection.name == name:
      return index
  -1

proc resolveSelection*(name, startDir: string,
                       configDir = ""): ToolSelection =
  let selections = visibleSelections(startDir, configDir)
  let index = selections.findSelection(name)
  if index < 0:
    raise newException(ValueError,
      "no version configured for " & name & " in tool version files")
  selections[index]
