package shell

import (
	"testing"
)

func TestFormatPATH(t *testing.T) {
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
		{Pwsh, "/usr/local/bin", "$env:PATH = \"/usr/local/bin\" + \";\" + $env:PATH\n"},
		{Powershell, "/usr/local/bin", "$env:PATH = \"/usr/local/bin\" + \";\" + $env:PATH\n"},
		{Nu, "/usr/local/bin", "{\"PATH\":\"/usr/local/bin\"}\n"},
		{Nushell, "/usr/local/bin", "{\"PATH\":\"/usr/local/bin\"}\n"},
	}

	for _, tt := range tests {
		got := FormatPATH(tt.shell, tt.dir)
		if got != tt.want {
			t.Errorf("FormatPATH(%q, %q) = %q, want %q", tt.shell, tt.dir, got, tt.want)
		}
	}
}

func TestFormatMultiPATH(t *testing.T) {
	dirs := []string{"/a", "/b"}

	got := FormatMultiPATH(Sh, dirs)
	want := "export PATH=\"/a\":\"/b\":$PATH\n"
	if got != want {
		t.Errorf("FormatMultiPATH(sh) = %q, want %q", got, want)
	}

	got = FormatMultiPATH(Fish, dirs)
	want = "set -gx PATH \"/a\" \"/b\" $PATH;\n"
	if got != want {
		t.Errorf("FormatMultiPATH(fish) = %q, want %q", got, want)
	}

	got = FormatMultiPATH(Pwsh, dirs)
	want = "$env:PATH = \"/a\" + \";\" + \"/b\" + \";\" + $env:PATH\n"
	if got != want {
		t.Errorf("FormatMultiPATH(pwsh) = %q, want %q", got, want)
	}

	got = FormatMultiPATH(Nu, dirs)
	want = "{\"PATH\":\"/a:/b\"}\n"
	if got != want {
		t.Errorf("FormatMultiPATH(nu) = %q, want %q", got, want)
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
