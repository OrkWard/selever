package install

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCheckVSArch(t *testing.T) {
	native := NativeVSArch()
	for _, c := range []struct {
		host, target, wantHost, wantTarget string
		ok                                 bool
	}{
		{"", "", native, native, true},
		{"X64", "", "x64", "x64", true},
		{"x64", "arm", "x64", "arm", true},
		{"arm", "x64", "", "", false},
		{"x64", "amd64", "", "", false},
	} {
		h, tg, err := CheckVSArch(c.host, c.target)
		if (err == nil) != c.ok || h != c.wantHost || tg != c.wantTarget {
			t.Errorf("CheckVSArch(%q, %q) = %q, %q, %v", c.host, c.target, h, tg, err)
		}
	}
}

func TestMSICabNames(t *testing.T) {
	data := []byte("xx0123456789abcdef0123456789ABCDEF.cab\x00short.cab\x00" +
		"fedcba9876543210fedcba9876543210.cab")
	path := filepath.Join(t.TempDir(), "a.msi")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := msiCabNames(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"0123456789abcdef0123456789abcdef.cab", "fedcba9876543210fedcba9876543210.cab"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("msiCabNames = %v, want %v", got, want)
	}
}

func TestPayloadBase(t *testing.T) {
	if got := payloadBase(`Installers\Windows SDK-x86_en-us.msi`); got != "Windows SDK-x86_en-us.msi" {
		t.Errorf("payloadBase = %q", got)
	}
}
