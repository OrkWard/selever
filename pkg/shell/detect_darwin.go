package shell

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func detectProc() string {
	out, err := exec.Command("ps", "-o", "comm=", "-p", strconv.Itoa(os.Getppid())).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.ToLower(string(out)))
}
