-- selever: luarocks.core.hardcoded for a Lua bundle. LuaRocks lives in
-- <bundle>/bin, as the standalone binary or as the script the bundle's lua
-- runs, and serves the Lua of that bundle; the paths follow from where it
-- runs. The build prepends `local static = {...}` with what the stock
-- module would hold (SYSTEM, PROCESSOR and IS_BINARY for the binaries).

local windows = package.config:sub(1, 1) == "\\"

local function program_path()
   -- Something in <bundle>/bin: on Linux the running executable, the
   -- binary or the bundle's lua; elsewhere arg[0], which is the executable
   -- on Windows (GetModuleFileName) and the luarocks script on macOS.
   if not windows then
      local ok, lfs = pcall(require, "lfs")
      local target = ok and lfs.symlinkattributes("/proc/self/exe", "target")
      if target then
         return target
      end
   end
   local path = arg and arg[0] or ""
   if not windows and path:sub(1, 1) ~= "/" then
      path = (os.getenv("PWD") or ".") .. "/" .. path
   end
   return path
end

local function parent(path)
   return path:match("^(.*)[/\\][^/\\]*$") or "."
end

local root = parent(parent(program_path()))
local exe = windows and ".exe" or ""

-- The bundle holds the configuration file of its own Lua version only.
local version
for _, candidate in ipairs({ "5.5", "5.4", "5.3", "5.2", "5.1" }) do
   local file = io.open(root .. "/etc/luarocks/config-" .. candidate .. ".lua")
   if file then
      file:close()
      version = candidate
      break
   end
end

local hardcoded = {
   SYSCONFDIR = root .. "/etc/luarocks",
   LUA_VERSION = version,
   LUA_DIR = root,
   LUA_BINDIR = root .. "/bin",
   LUA_INCDIR = root .. "/include",
   LUA_LIBDIR = root .. (windows and "/bin" or "/lib"),
   LUA = root .. "/bin/lua" .. exe,
}
for key, value in pairs(static) do
   hardcoded[key] = value
end
return hardcoded
