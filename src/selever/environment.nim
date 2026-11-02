import std/[os, strtabs, tables]

type
  EnvironmentChanges* = object
    ## Variables to set and list entries to prepend, in priority order.
    variables*: OrderedTable[string, string]
    prepends*: OrderedTable[string, seq[string]]

proc setVariable*(changes: var EnvironmentChanges, name, value: string) =
  changes.variables[name] = value

proc prepend*(changes: var EnvironmentChanges, name: string,
              entries: varargs[string]) =
  ## Earlier entries end up first.
  var current = changes.prepends.getOrDefault(name)
  for entry in entries:
    current.add entry
  changes.prepends[name] = current

proc merge*(changes: var EnvironmentChanges, other: EnvironmentChanges) =
  ## Lower priority: other's lists go after ours, and its variables only fill
  ## names we don't set.
  for name, value in other.variables:
    if name notin changes.variables:
      changes.setVariable(name, value)
  for name, entries in other.prepends:
    changes.prepend(name, entries)

proc applyTo*(changes: EnvironmentChanges, environment: StringTableRef) =
  for name, entries in changes.prepends:
    var value = ""
    for entry in entries:
      if value.len > 0:
        value.add PathSep
      value.add entry
    let existing = environment.getOrDefault(name)
    if existing.len > 0:
      value.add PathSep
      value.add existing
    environment[name] = value
  for name, value in changes.variables:
    environment[name] = value
