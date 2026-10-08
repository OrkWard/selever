package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
)

// MSIAdminExtract unpacks an MSI into targetDir with an administrative
// install, which needs no elevation and registers nothing. The MSI's external
// CABs must sit next to it.
func MSIAdminExtract(msi, targetDir string) error {
	msi, err := filepath.Abs(msi)
	if err != nil {
		return err
	}
	targetDir, err = filepath.Abs(targetDir)
	if err != nil {
		return err
	}
	log := msi + ".log"

	// msiexec parses PROPERTY="value" itself, so build the command line by
	// hand instead of letting Go quote whole arguments.
	cmd := exec.Command("msiexec.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: fmt.Sprintf(`msiexec.exe /a "%s" /quiet /qn /le "%s" TARGETDIR="%s"`,
			msi, log, strings.TrimRight(targetDir, `\`)),
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("msiexec: %w%s", err, readMSILog(log))
	}
	return nil
}

// readMSILog returns the error log msiexec wrote, which is UTF-16LE.
func readMSILog(path string) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 2 {
		return ""
	}
	if data[0] == 0xff && data[1] == 0xfe {
		data = data[2:]
	}
	u := make([]uint16, len(data)/2)
	for i := range u {
		u[i] = uint16(data[2*i]) | uint16(data[2*i+1])<<8
	}
	s := strings.TrimSpace(string(utf16.Decode(u)))
	if s == "" {
		return ""
	}
	return "\n" + s
}
