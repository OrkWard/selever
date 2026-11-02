import std/[monotimes, os, syncio, times]

when defined(windows):
  import std/winlean

  type Overlapped = object
    internal: ULONG_PTR
    internalHigh: ULONG_PTR
    offset: DWORD
    offsetHigh: DWORD
    event: Handle

  proc lockFileEx(handle: Handle, flags, reserved, low, high: DWORD,
                  overlapped: ptr Overlapped): WINBOOL
                  {.stdcall, dynlib: "kernel32", importc: "LockFileEx".}
  proc unlockFileEx(handle: Handle, reserved, low, high: DWORD,
                    overlapped: ptr Overlapped): WINBOOL
                    {.stdcall, dynlib: "kernel32", importc: "UnlockFileEx".}

  const
    LockfileExclusiveLock = DWORD(0x00000002)
    LockfileFailImmediately = DWORD(0x00000001)
    ErrorSharingViolation = 32
    ErrorLockViolation = 33
else:
  import std/posix

  proc flock(handle: cint, operation: cint): cint
             {.importc, header: "<sys/file.h>".}

  const
    LockExclusive = cint(2)
    LockNonBlocking = cint(4)
    LockUnlock = cint(8)

type InstallLock* = object
  file: File
  acquired: bool
  when defined(windows):
    overlapped: Overlapped

proc tryAcquire(lock: var InstallLock): bool =
  when defined(windows):
    let handle = cast[Handle](getOsFileHandle(lock.file))
    if lockFileEx(
      handle,
      LockfileExclusiveLock or LockfileFailImmediately,
      0,
      high(DWORD),
      high(DWORD),
      addr lock.overlapped,
    ) != 0:
      return true
    let code = osLastError()
    if code.int notin [ErrorSharingViolation, ErrorLockViolation]:
      raiseOSError(code)
  else:
    if flock(getFileHandle(lock.file), LockExclusive or LockNonBlocking) == 0:
      return true
    let code = osLastError()
    if code.int notin [EAGAIN.int, EWOULDBLOCK.int]:
      raiseOSError(code)

proc acquireInstallLock*(path: string; timeoutMs = 30_000): InstallLock =
  createDir(parentDir(path))
  if not fileExists(path):
    writeFile(path, "")
  if not open(result.file, path, fmReadWriteExisting):
    raise newException(IOError, "could not open install lock: " & path)

  let deadline = getMonoTime() + initDuration(milliseconds = timeoutMs)
  while true:
    if result.tryAcquire():
      result.acquired = true
      return
    if getMonoTime() >= deadline:
      close(result.file)
      raise newException(IOError, "timed out acquiring install lock: " & path)
    sleep(100)

proc release*(lock: var InstallLock) =
  if not lock.acquired:
    return
  when defined(windows):
    let handle = cast[Handle](getOsFileHandle(lock.file))
    if unlockFileEx(
      handle,
      0,
      high(DWORD),
      high(DWORD),
      addr lock.overlapped,
    ) == 0:
      let code = osLastError()
      close(lock.file)
      lock.acquired = false
      raiseOSError(code)
  else:
    if flock(getFileHandle(lock.file), LockUnlock) != 0:
      let code = osLastError()
      close(lock.file)
      lock.acquired = false
      raiseOSError(code)
  close(lock.file)
  lock.acquired = false
