package globver

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writePackage(t *testing.T, modules, name, pkgJSON string) {
	t.Helper()
	dir := filepath.Join(modules, filepath.FromSlash(name))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNpmPackageExes(t *testing.T) {
	modules := filepath.Join(t.TempDir(), "node_modules")
	binDir := filepath.Join(modules, ".bin")
	writePackage(t, modules, "@scope/tool", `{"name":"@scope/tool","bin":{"tool":"cli.js","tool-ai":"ai.js"}}`)
	writePackage(t, modules, "single", `{"name":"single","bin":"index.js"}`)
	writePackage(t, modules, "lib", `{"name":"lib"}`)

	tests := []struct {
		pkg  string
		want []string
	}{
		{"@scope/tool", []string{"tool", "tool-ai"}},
		{"single", []string{"single"}},
	}
	for _, tt := range tests {
		got, err := npmPackageExes(binDir, tt.pkg)
		if err != nil {
			t.Fatalf("npmPackageExes(%q): %v", tt.pkg, err)
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("npmPackageExes(%q) = %v, want %v", tt.pkg, got, tt.want)
		}
	}

	if _, err := npmPackageExes(binDir, "lib"); err == nil {
		t.Error("npmPackageExes(lib) succeeded for a package without bin")
	}
}
