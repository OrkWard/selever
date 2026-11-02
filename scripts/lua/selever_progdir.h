/*
** Included by build-lua.sh in place of loadlib.c's no-op setprogdir.
** On Windows Lua replaces LUA_EXEC_DIR ("!") in package.path and
** package.cpath with the executable's directory; this does the same on
** Linux and macOS, so a bundle finds its modules wherever it is unpacked.
*/

#if defined(_WIN32)

#define setprogdir(L)		((void)0)

#else

#include <limits.h>
#include <stdlib.h>
#include <string.h>

#if defined(__APPLE__)
#include <mach-o/dyld.h>
#include <stdint.h>
#endif

#if !defined(PATH_MAX)
#define PATH_MAX 4096
#endif

#if defined(LUA_EXECDIR) && !defined(LUA_EXEC_DIR)
#define LUA_EXEC_DIR		LUA_EXECDIR
#endif

static void setprogdir (lua_State *L) {
  char real[PATH_MAX];
  char *lb;
#if defined(__APPLE__)
  char buff[PATH_MAX];
  uint32_t size = sizeof(buff);
  if (_NSGetExecutablePath(buff, &size) != 0 || realpath(buff, real) == NULL)
#else
  if (realpath("/proc/self/exe", real) == NULL)
#endif
    luaL_error(L, "unable to locate the Lua executable");
  else if ((lb = strrchr(real, '/')) == NULL)
    luaL_error(L, "unable to locate the Lua executable");
  else {
    *lb = '\0';
    luaL_gsub(L, lua_tostring(L, -1), LUA_EXEC_DIR, real);
    lua_remove(L, -2);
  }
}

#endif
