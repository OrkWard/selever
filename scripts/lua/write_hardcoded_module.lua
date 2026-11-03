local function write_hardcoded_module(dir)
   -- selever: instead of fixed paths, the binaries get scripts/lua/hardcoded.lua,
   -- which finds the bundle they are in.
   local system, processor
   if if_platform("unix", true) then
      system, processor = sysdetect.detect()
   else
      system, processor = "windows", PROCESSOR
   end
   local template = assert(io.open(assert(os.getenv("SELEVER_HARDCODED")), "rb"))
   local name = dir .. "/luarocks/core/hardcoded.lua"
   local out = assert(io.open(name, "wb"))
   out:write(("local static = { SYSTEM = %q, PROCESSOR = %q, IS_BINARY = true }\n")
      :format(system, processor))
   out:write(template:read("*a"))
   template:close()
   out:close()
   return name
end
