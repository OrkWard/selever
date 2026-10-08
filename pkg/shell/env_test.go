package shell

import (
	"os"
	"strings"
	"testing"
)

func testEnv() *Env {
	e := &Env{}
	e.Prepend("PATH", `C:\vc\bin`)
	e.Prepend("INCLUDE", `C:\vc\include`)
	e.Set("VCToolsVersion", "14.44.35207")

	sdk := &Env{}
	sdk.Prepend("Path", `C:\sdk\bin`)
	sdk.Prepend("INCLUDE", `C:\sdk\ucrt`, `C:\sdk\um`)
	sdk.Set("WindowsSDKVersion", `10.0.26100.0\`)
	sdk.Set("VCToolsVersion", "14.44.1")
	e.Merge(sdk)
	return e
}

func TestFormatEnv(t *testing.T) {
	sep := string(os.PathListSeparator)
	inc := `C:\vc\include` + sep + `C:\sdk\ucrt` + sep + `C:\sdk\um`

	tests := []struct {
		shell Name
		want  []string
	}{
		{Sh, []string{
			`export PATH="C:\\vc\\bin":"C:\\sdk\\bin":$PATH`,
			`export INCLUDE="` + strings.ReplaceAll(inc, `\`, `\\`) + `"${INCLUDE:+"` + sep + `"$INCLUDE}`,
			`export VCToolsVersion="14.44.1"`,
			`export WindowsSDKVersion="10.0.26100.0\\"`,
		}},
		{Fish, []string{
			`set -gx PATH "C:\\vc\\bin" "C:\\sdk\\bin" $PATH;`,
			`set -gx INCLUDE "` + strings.ReplaceAll(inc, `\`, `\\`) + sep + `""$INCLUDE";`,
			`set -gx VCToolsVersion "14.44.1";`,
			`set -gx WindowsSDKVersion "10.0.26100.0\\";`,
		}},
		{Pwsh, []string{
			`$env:PATH = 'C:\vc\bin' + ';' + 'C:\sdk\bin' + ';' + $env:PATH`,
			`$env:INCLUDE = 'C:\vc\include' + ';' + 'C:\sdk\ucrt' + ';' + 'C:\sdk\um' + ';' + $env:INCLUDE`,
			`$env:VCToolsVersion = '14.44.1'`,
			`$env:WindowsSDKVersion = '10.0.26100.0\'`,
		}},
	}
	for _, tt := range tests {
		got := FormatEnv(tt.shell, testEnv())
		want := strings.Join(tt.want, "\n") + "\n"
		if got != want {
			t.Errorf("FormatEnv(%s) =\n%s\nwant\n%s", tt.shell, got, want)
		}
	}
}

func TestFormatEnvNu(t *testing.T) {
	sep := string(os.PathListSeparator)
	t.Setenv("PATH", "/old")
	t.Setenv("INCLUDE", "/inc")

	got := FormatEnv(Nu, testEnv())
	want := `{"INCLUDE":"C:\\vc\\include` + sep + `C:\\sdk\\ucrt` + sep + `C:\\sdk\\um` + sep + `/inc",` +
		`"PATH":["C:\\vc\\bin","C:\\sdk\\bin","/old"],` +
		`"VCToolsVersion":"14.44.1","WindowsSDKVersion":"10.0.26100.0\\"}` + "\n"
	if got != want {
		t.Errorf("FormatEnv(nu) = %s, want %s", got, want)
	}
}

func TestPwshQuote(t *testing.T) {
	if got, want := pwshQuote(`C:\it's $x`), `'C:\it''s $x'`; got != want {
		t.Errorf("pwshQuote = %s, want %s", got, want)
	}
}
