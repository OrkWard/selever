package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/orkward/selever/pkg/shell"
)

// MSVCDir returns the installation directory of an MSVC toolset.
func MSVCDir(p *Paths, version, host, target string) string {
	return p.InstallDir("msvc", version+"-"+host+"-"+target)
}

// InstallMSVC installs the MSVC toolset with the exact manifest version (for
// example "14.44.17.14" or "14.51") that runs on host and builds for target,
// and returns its environment.
func InstallMSVC(ctx context.Context, version, host, target string) (*shell.Env, error) {
	if err := requireWindows("MSVC"); err != nil {
		return nil, err
	}
	host, target, err := CheckVSArch(host, target)
	if err != nil {
		return nil, err
	}
	version = strings.ToLower(version)

	p, err := ResolvePaths()
	if err != nil {
		return nil, err
	}
	dir := MSVCDir(p, version, host, target)
	if info, err := readVSInfo(dir); err == nil {
		return msvcEnv(dir, info), nil
	}

	lock, err := AcquireLock(LockPath(dir), vsLockTimeout)
	if err != nil {
		return nil, err
	}
	defer lock.Release()
	if info, err := readVSInfo(dir); err == nil {
		return msvcEnv(dir, info), nil // installed while waiting
	}

	toolsID := msvcToolsID(version, host, target)
	m, err := loadVSManifestFor(func(m *VSManifest) bool { return m.Lookup(toolsID) != nil })
	if err != nil {
		return nil, err
	}
	payloads, err := msvcPayloads(m, version, host, target)
	if err != nil {
		return nil, err
	}
	files, err := fetchPayloads(ctx, p.CacheDir("msvc"), payloads)
	if err != nil {
		return nil, err
	}

	staging := dir + ".staging"
	os.RemoveAll(staging)
	info, err := buildMSVC(staging, files, host, target)
	if err != nil {
		os.RemoveAll(staging)
		return nil, err
	}
	if err := finishStaging(staging, dir); err != nil {
		os.RemoveAll(staging)
		return nil, err
	}
	return msvcEnv(dir, info), nil
}

// msvcPayloads lists the payloads of the packages that make up a toolset:
// compiler, its English resources, CRT headers and libraries, ASan, PGO, and
// the debug CRT DLLs from the redistributable package.
func msvcPayloads(m *VSManifest, v, host, target string) ([]VSPayload, error) {
	pre := "microsoft.vc." + v + "."
	required := []string{
		msvcToolsID(v, host, target),
		pre + "crt.headers.base",
		pre + "crt." + target + ".desktop.base",
		// Despite its name this also holds msvcrt.lib, oldnames.lib, and the
		// other import libraries; its UWP-only store/ subdirectory is removed.
		pre + "crt." + target + ".store.base",
	}
	optional := []string{
		fmt.Sprintf("%stools.host%s.target%s.res.base", pre, host, target),
		fmt.Sprintf("%spremium.tools.host%s.target%s.base", pre, host, target),
		pre + "asan.headers.base",
		pre + "pgo.headers.base",
		pre + "pgo." + target + ".base",
	}
	if target == "x64" || target == "x86" {
		optional = append(optional, pre+"asan."+target+".base")
	}
	if target == "arm" {
		optional = append(optional, pre+"crt.redist.arm.onecore.desktop.base")
	} else {
		optional = append(optional, pre+"crt.redist."+target+".base")
	}

	var payloads []VSPayload
	for _, id := range required {
		pkg := m.Lookup(id)
		if pkg == nil {
			return nil, fmt.Errorf("MSVC %s (host %s, target %s) not found: missing package %s", v, host, target, id)
		}
		payloads = append(payloads, pkg.Payloads...)
	}
	for _, id := range optional {
		if pkg := m.Lookup(id); pkg != nil {
			payloads = append(payloads, pkg.Payloads...)
		}
	}
	return payloads, nil
}

func buildMSVC(staging string, files []string, host, target string) (*vsInfo, error) {
	for _, f := range files {
		if err := extractVSIX(f, staging); err != nil {
			return nil, fmt.Errorf("extract: %w", err)
		}
	}

	hostDir := "Host" + host
	toolsRoot := filepath.Join(staging, "VC", "Tools", "MSVC")
	vctools, err := latestVersionDir(toolsRoot, func(d string) bool {
		return exists(filepath.Join(d, "include")) && exists(filepath.Join(d, "bin", hostDir, target, "cl.exe"))
	})
	if err != nil {
		return nil, err
	}
	tools := filepath.Join(toolsRoot, vctools)
	bin := filepath.Join(tools, "bin", hostDir, target)

	// Debug CRT DLLs go next to the compiler so /MDd programs run.
	redist := filepath.Join(staging, "VC", "Redist")
	debug, _ := filepath.Glob(filepath.Join(redist, "MSVC", "*", "debug_nonredist", target, "*", "*.dll"))
	for _, dll := range debug {
		if err := os.Rename(dll, filepath.Join(bin, filepath.Base(dll))); err != nil {
			return nil, err
		}
	}

	for _, junk := range []string{
		redist,
		filepath.Join(staging, "Common7"),
		filepath.Join(tools, "Auxiliary"),
		filepath.Join(tools, "lib", target, "store"),
		filepath.Join(tools, "lib", target, "uwp"),
		filepath.Join(tools, "lib", target, "enclave"),
		filepath.Join(tools, "lib", target, "onecore"),
		filepath.Join(bin, "onecore"),
		filepath.Join(bin, "vctip.exe"),
		filepath.Join(bin, "Microsoft.VisualStudio.Telemetry.dll"),
	} {
		if err := os.RemoveAll(junk); err != nil {
			return nil, err
		}
	}

	// Tools that locate MSVC through vcvars read this file.
	build := filepath.Join(staging, "VC", "Auxiliary", "Build")
	if err := os.MkdirAll(build, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(build, "Microsoft.VCToolsVersion.default.txt"), []byte(vctools+"\n"), 0o644); err != nil {
		return nil, err
	}

	info := &vsInfo{Version: vctools, Host: host, Target: target}
	return info, writeVSInfo(staging, info)
}

func msvcEnv(dir string, info *vsInfo) *shell.Env {
	vc := filepath.Join(dir, "VC")
	tools := filepath.Join(vc, "Tools", "MSVC", info.Version)
	lib := filepath.Join(tools, "lib", info.Target)

	e := &shell.Env{}
	e.Prepend("PATH", filepath.Join(tools, "bin", "Host"+info.Host, info.Target))
	e.Prepend("INCLUDE", filepath.Join(tools, "include"))
	e.Prepend("LIB", lib)
	e.Prepend("LIBPATH", lib)
	e.Set("VCINSTALLDIR", vc+`\`)
	e.Set("VCToolsInstallDir", tools+`\`)
	e.Set("VCToolsVersion", info.Version)
	e.Set("VSCMD_ARG_HOST_ARCH", info.Host)
	e.Set("VSCMD_ARG_TGT_ARCH", info.Target)
	return e
}
