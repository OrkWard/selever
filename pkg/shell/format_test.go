package shell

import (
	"os"
	"strings"
	"testing"
)

func TestFormatPathEnv(t *testing.T) {
	sep := string(os.PathListSeparator)
	t.Setenv("PATH", strings.Join([]string{"/old1", "/old2"}, sep))
	pathKey := envName("PATH") // Path on Windows

	tests := []struct {
		shell Name
		dir   string
		want  string
	}{
		{Sh, "/usr/local/bin", "export PATH=\"/usr/local/bin\":$PATH\n"},
		{Bash, "/usr/local/bin", "export PATH=\"/usr/local/bin\":$PATH\n"},
		{Zsh, "/usr/local/bin", "export PATH=\"/usr/local/bin\":$PATH\n"},
		{Auto, "/usr/local/bin", "export PATH=\"/usr/local/bin\":$PATH\n"},
		{Fish, "/usr/local/bin", "set -gx PATH \"/usr/local/bin\" $PATH;\n"},
		{Pwsh, "/usr/local/bin", "$env:PATH = '/usr/local/bin' + ';' + $env:PATH\n"},
		{Powershell, "/usr/local/bin", "$env:PATH = '/usr/local/bin' + ';' + $env:PATH\n"},
		{Nu, "/usr/local/bin", "{\"" + pathKey + "\":[\"/usr/local/bin\",\"/old1\",\"/old2\"]}\n"},
		{Nushell, "/usr/local/bin", "{\"" + pathKey + "\":[\"/usr/local/bin\",\"/old1\",\"/old2\"]}\n"},
	}

	for _, tt := range tests {
		got := FormatEnv(tt.shell, PathEnv(tt.dir))
		if got != tt.want {
			t.Errorf("FormatEnv(PathEnv)(%q, %q) = %q, want %q", tt.shell, tt.dir, got, tt.want)
		}
	}
}

func TestFormatPathEnvMulti(t *testing.T) {
	t.Setenv("PATH", "/old")
	dirs := []string{"/a", "/b"}

	got := FormatEnv(Sh, PathEnv(dirs...))
	want := "export PATH=\"/a\":\"/b\":$PATH\n"
	if got != want {
		t.Errorf("FormatEnv(sh) = %q, want %q", got, want)
	}

	got = FormatEnv(Fish, PathEnv(dirs...))
	want = "set -gx PATH \"/a\" \"/b\" $PATH;\n"
	if got != want {
		t.Errorf("FormatEnv(fish) = %q, want %q", got, want)
	}

	got = FormatEnv(Pwsh, PathEnv(dirs...))
	want = "$env:PATH = '/a' + ';' + '/b' + ';' + $env:PATH\n"
	if got != want {
		t.Errorf("FormatEnv(pwsh) = %q, want %q", got, want)
	}

	got = FormatEnv(Nu, PathEnv(dirs...))
	want = "{\"" + envName("PATH") + "\":[\"/a\",\"/b\",\"/old\"]}\n"
	if got != want {
		t.Errorf("FormatEnv(nu) = %q, want %q", got, want)
	}
}

func TestCanonical(t *testing.T) {
	tests := []struct {
		in   string
		want Name
	}{
		{"sh", Sh},
		{"bash", Bash},
		{"zsh", Zsh},
		{"fish", Fish},
		{"pwsh", Pwsh},
		{"powershell", Pwsh},
		{"nu", Nu},
		{"nushell", Nu},
		{"auto", Auto},
		{"", ""},
		{"unknown", ""},
	}

	for _, tt := range tests {
		got := Canonical(tt.in)
		if got != tt.want {
			t.Errorf("Canonical(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
