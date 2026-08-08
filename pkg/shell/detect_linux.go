package shell

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func detectProc() string {
	ppid := os.Getppid()
	comm, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(ppid), "comm"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.ToLower(string(comm)))
}
