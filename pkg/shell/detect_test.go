package shell

import (
	"os"
	"testing"
)

func TestShellFromEnv(t *testing.T) {
	// Save and restore SHELL.
	orig := os.Getenv("SHELL")
	defer os.Setenv("SHELL", orig)

	tests := []struct {
		shellEnv string
		want     string
	}{
		{"/bin/bash", "bash"},
		{"/usr/bin/zsh", "zsh"},
		{"/usr/bin/fish", "fish"},
		{"/bin/sh", "sh"},
		{"", ""},
	}

	for _, tt := range tests {
		os.Setenv("SHELL", tt.shellEnv)
		got := shellFromEnv()
		if got != tt.want {
			t.Errorf("shellFromEnv() with SHELL=%q = %q, want %q", tt.shellEnv, got, tt.want)
		}
	}
}

func TestDetectShellFallback(t *testing.T) {
	// DetectShell always returns a valid shell (falling back to Sh).
	got := DetectShell()
	if Canonical(string(got)) == "" {
		t.Errorf("DetectShell() returned invalid shell: %q", got)
	}
}
