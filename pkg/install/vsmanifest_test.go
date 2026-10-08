package install

import (
	"encoding/json"
	"reflect"
	"testing"
)

const vsManifestFixture = `{"packages": [
  {"id": "Microsoft.VC.14.44.17.14.Tools.HostX64.TargetX64.base", "version": "14.44.35229"},
  {"id": "Microsoft.VC.14.44.17.14.Tools.HostX64.TargetX86.base", "version": "14.44.35229"},
  {"id": "Microsoft.VC.14.9.16.11.Tools.HostX64.TargetX64.base", "version": "14.9.1"},
  {"id": "Microsoft.VC.14.52.Tools.HostX64.TargetX64.base", "version": "14.52.36725"},
  {"id": "Microsoft.VC.14.52.Tools.HostX64.TargetX64.Res.base", "language": "de-DE"},
  {"id": "Microsoft.VC.14.52.Tools.HostX64.TargetX64.Res.base", "language": "en-US"},
  {"id": "Microsoft.VisualStudio.Component.Windows11SDK.22621", "dependencies": {"Win11SDK_10.0.22621": ""}},
  {"id": "Microsoft.VisualStudio.Component.Windows11SDK.26100", "dependencies": {"Win11SDK_10.0.26100": ""}},
  {"id": "Win11SDK_10.0.22621", "payloads": [{"fileName": "Installers\\a.msi"}]},
  {"id": "Win11SDK_10.0.26100", "payloads": [{"fileName": "Installers\\b.msi"}]}
]}`

func testVSManifest(t *testing.T) *VSManifest {
	t.Helper()
	m := &VSManifest{}
	if err := json.Unmarshal([]byte(vsManifestFixture), m); err != nil {
		t.Fatal(err)
	}
	m.index()
	return m
}

func TestMSVCVersions(t *testing.T) {
	m := testVSManifest(t)
	got := m.MSVCVersions("x64", "x64")
	want := []MSVCVersion{
		{"14.52", "14.52.36725"},
		{"14.44.17.14", "14.44.35229"},
		{"14.9.16.11", "14.9.1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MSVCVersions = %v, want %v", got, want)
	}
	if got := m.MSVCVersions("x64", "arm64"); len(got) != 0 {
		t.Errorf("MSVCVersions(arm64) = %v, want none", got)
	}
}

func TestVSLookupPrefersEnglish(t *testing.T) {
	p := testVSManifest(t).Lookup("microsoft.vc.14.52.tools.hostx64.targetx64.res.base")
	if p == nil || p.Language != "en-US" {
		t.Errorf("Lookup = %+v, want en-US variant", p)
	}
}

func TestWinSDKs(t *testing.T) {
	m := testVSManifest(t)
	sdks := m.WinSDKs()
	if len(sdks) != 2 || sdks[0].Build != "26100" || sdks[0].Package.ID != "Win11SDK_10.0.26100" || sdks[1].Build != "22621" {
		t.Errorf("WinSDKs = %+v", sdks)
	}
	if s := m.WinSDK("22621"); s == nil || s.Package.ID != "Win11SDK_10.0.22621" {
		t.Errorf("WinSDK(22621) = %+v", s)
	}
	if s := m.WinSDK("19041"); s != nil {
		t.Errorf("WinSDK(19041) = %+v, want nil", s)
	}
}

func TestCompareVersions(t *testing.T) {
	for _, c := range []struct {
		a, b string
		sign int
	}{
		{"14.52", "14.44.17.14", 1},
		{"14.44.17.14", "14.44.17.9", 1},
		{"10.0.26100.0", "10.0.26100.0", 0},
		{"14.9", "14.10", -1},
	} {
		got := compareVersions(c.a, c.b)
		if (got > 0) != (c.sign > 0) || (got < 0) != (c.sign < 0) {
			t.Errorf("compareVersions(%s, %s) = %d, want sign %d", c.a, c.b, got, c.sign)
		}
	}
}
