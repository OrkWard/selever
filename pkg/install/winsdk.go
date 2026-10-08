package install

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/orkward/selever/pkg/shell"
	"github.com/orkward/selever/pkg/utils"
)

// WinSDKDir returns the installation directory of a Windows SDK.
func WinSDKDir(p *Paths, build, host, target string) string {
	return p.InstallDir("winsdk", build+"-"+host+"-"+target)
}

// InstallWinSDK installs the Windows SDK with the exact build number (for
// example "26100") with tools for host and libraries for target, and returns
// its environment.
func InstallWinSDK(ctx context.Context, build, host, target string) (*shell.Env, error) {
	if err := requireWindows("the Windows SDK"); err != nil {
		return nil, err
	}
	host, target, err := CheckVSArch(host, target)
	if err != nil {
		return nil, err
	}

	p, err := ResolvePaths()
	if err != nil {
		return nil, err
	}
	dir := WinSDKDir(p, build, host, target)
	if info, err := readVSInfo(dir); err == nil {
		return winSDKEnv(dir, info), nil
	}

	lock, err := AcquireLock(LockPath(dir), vsLockTimeout)
	if err != nil {
		return nil, err
	}
	defer lock.Release()
	if info, err := readVSInfo(dir); err == nil {
		return winSDKEnv(dir, info), nil // installed while waiting
	}

	m, err := loadVSManifestFor(func(m *VSManifest) bool { return m.WinSDK(build) != nil })
	if err != nil {
		return nil, err
	}
	sdk := m.WinSDK(build)
	if sdk == nil {
		return nil, fmt.Errorf("Windows SDK %s not found", build)
	}

	byName := map[string]VSPayload{}
	for _, pl := range sdk.Package.Payloads {
		byName[strings.ToLower(payloadBase(pl.FileName))] = pl
	}
	msis, err := winSDKMSIs(byName, target)
	if err != nil {
		return nil, err
	}
	cacheDir := p.CacheDir("winsdk")
	msiFiles, err := fetchPayloads(ctx, cacheDir, msis)
	if err != nil {
		return nil, err
	}

	// Each MSI reads its CABs from the directory it sits in.
	var cabs []VSPayload
	seen := map[string]bool{}
	for _, f := range msiFiles {
		names, err := msiCabNames(f)
		if err != nil {
			return nil, err
		}
		for _, n := range names {
			if pl, ok := byName[n]; ok && !seen[n] {
				seen[n] = true
				cabs = append(cabs, pl)
			}
		}
	}
	cabFiles, err := fetchPayloads(ctx, cacheDir, cabs)
	if err != nil {
		return nil, err
	}

	staging := dir + ".staging"
	work := dir + ".work"
	os.RemoveAll(staging)
	os.RemoveAll(work)
	defer os.RemoveAll(work)

	info, err := buildWinSDK(staging, work, msiFiles, cabFiles, host, target)
	if err != nil {
		os.RemoveAll(staging)
		return nil, err
	}
	if err := finishStaging(staging, dir); err != nil {
		os.RemoveAll(staging)
		return nil, err
	}
	return winSDKEnv(dir, info), nil
}

// winSDKMSIs selects the MSIs with the headers, libraries, and tools needed
// to build desktop programs for target.
func winSDKMSIs(byName map[string]VSPayload, target string) ([]VSPayload, error) {
	required := []string{
		"Windows SDK for Windows Store Apps Tools-x86_en-us.msi",
		"Windows SDK for Windows Store Apps Headers-x86_en-us.msi",
		"Windows SDK for Windows Store Apps Headers OnecoreUap-x86_en-us.msi",
		"Windows SDK for Windows Store Apps Libs-x86_en-us.msi",
		"Universal CRT Headers Libraries and Sources-x86_en-us.msi",
		"Windows SDK Desktop Libs " + target + "-x86_en-us.msi",
	}
	// Desktop headers are split by architecture; take all that exist.
	var optional []string
	for _, a := range vsTargets {
		optional = append(optional,
			"Windows SDK Desktop Headers "+a+"-x86_en-us.msi",
			"Windows SDK OnecoreUap Headers "+a+"-x86_en-us.msi")
	}

	var out []VSPayload
	for _, n := range required {
		pl, ok := byName[strings.ToLower(n)]
		if !ok {
			return nil, fmt.Errorf("Windows SDK payload %q not found", n)
		}
		out = append(out, pl)
	}
	for _, n := range optional {
		if pl, ok := byName[strings.ToLower(n)]; ok {
			out = append(out, pl)
		}
	}
	return out, nil
}

// msiCabNames returns the lower-case names of the external CABs an MSI refers
// to. The SDK names them by a 32-digit hex hash, stored as plain ASCII.
func msiCabNames(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var names []string
	for i := 0; ; {
		j := bytes.Index(data[i:], []byte(".cab"))
		if j < 0 {
			return names, nil
		}
		end := i + j
		i = end + 4
		if end < 32 || !isHex(data[end-32:end]) {
			continue
		}
		names = append(names, strings.ToLower(string(data[end-32:end]))+".cab")
	}
}

func isHex(b []byte) bool {
	for _, c := range b {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func buildWinSDK(staging, work string, msiFiles, cabFiles []string, host, target string) (*vsInfo, error) {
	if err := os.MkdirAll(work, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return nil, err
	}
	for _, f := range append(append([]string{}, msiFiles...), cabFiles...) {
		if err := linkOrCopy(f, filepath.Join(work, filepath.Base(f))); err != nil {
			return nil, err
		}
	}
	for _, f := range msiFiles {
		name := filepath.Base(f)
		fmt.Fprintf(os.Stderr, "Extracting %s\n", name)
		if err := utils.MSIAdminExtract(filepath.Join(work, name), staging); err != nil {
			return nil, fmt.Errorf("extract %s: %w", name, err)
		}
		// An administrative install also copies the MSI into the target.
		os.Remove(filepath.Join(staging, name))
	}

	kits := filepath.Join(staging, "Windows Kits", "10")
	ver, err := latestVersionDir(filepath.Join(kits, "Include"), func(d string) bool {
		return exists(filepath.Join(d, "um", "windows.h"))
	})
	if err != nil {
		return nil, err
	}

	junk := []string{
		filepath.Join(kits, "Catalogs"),
		filepath.Join(kits, "DesignTime"),
		filepath.Join(kits, "bin", ver, "chpe"),
		filepath.Join(kits, "Lib", ver, "ucrt_enclave"),
	}
	for _, a := range vsTargets {
		if a != target {
			junk = append(junk,
				filepath.Join(kits, "Lib", ver, "ucrt", a),
				filepath.Join(kits, "Lib", ver, "um", a))
		}
		if a == host {
			continue
		}
		binDir := filepath.Join(kits, "bin", ver, a)
		if a != target {
			junk = append(junk, binDir)
			continue
		}
		// Keep the target's debug UCRT (ucrtbased.dll) to run /MDd programs.
		entries, _ := os.ReadDir(binDir)
		for _, e := range entries {
			if e.Name() != "ucrt" {
				junk = append(junk, filepath.Join(binDir, e.Name()))
			}
		}
	}
	for _, j := range junk {
		if err := os.RemoveAll(j); err != nil {
			return nil, err
		}
	}

	info := &vsInfo{Version: ver, Host: host, Target: target}
	return info, writeVSInfo(staging, info)
}

func linkOrCopy(src, dst string) error {
	if err := os.Link(src, dst); err == nil {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func winSDKEnv(dir string, info *vsInfo) *shell.Env {
	kits := filepath.Join(dir, "Windows Kits", "10")
	inc := filepath.Join(kits, "Include", info.Version)
	lib := filepath.Join(kits, "Lib", info.Version)
	bin := filepath.Join(kits, "bin", info.Version)

	e := &shell.Env{}
	// Host tools, then the debug UCRT that target programs load.
	e.Prepend("PATH", filepath.Join(bin, info.Host), filepath.Join(bin, info.Target, "ucrt"))
	for _, sub := range []string{"ucrt", "shared", "um", "winrt", "cppwinrt"} {
		e.Prepend("INCLUDE", filepath.Join(inc, sub))
	}
	e.Prepend("LIB", filepath.Join(lib, "ucrt", info.Target), filepath.Join(lib, "um", info.Target))
	e.Set("WindowsSdkDir", kits+`\`)
	e.Set("WindowsSDKVersion", info.Version+`\`)
	e.Set("WindowsSDKLibVersion", info.Version+`\`)
	e.Set("WindowsSdkBinPath", filepath.Join(kits, "bin")+`\`)
	e.Set("WindowsSdkVerBinPath", filepath.Join(kits, "bin", info.Version)+`\`)
	e.Set("UniversalCRTSdkDir", kits+`\`)
	e.Set("UCRTVersion", info.Version)
	return e
}
