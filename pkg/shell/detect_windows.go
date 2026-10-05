package shell

import (
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// detectProc returns the parent process's image name without ".exe".
func detectProc() string {
	ppid := uint32(os.Getppid())
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return ""
	}
	defer syscall.CloseHandle(snap)

	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = syscall.Process32First(snap, &e); err == nil; err = syscall.Process32Next(snap, &e) {
		if e.ProcessID == ppid {
			name := strings.ToLower(syscall.UTF16ToString(e.ExeFile[:]))
			return strings.TrimSuffix(name, ".exe")
		}
	}
	return ""
}
