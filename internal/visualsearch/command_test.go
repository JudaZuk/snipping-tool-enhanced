// SPDX-License-Identifier: Apache-2.0

package visualsearch

import "testing"

func TestCommand(t *testing.T) {
	edge := `"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe" --single-argument %1`
	exe, _ := splitCommand(edge)
	if exe != `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe` {
		t.Fatal(exe)
	}
	if got := fillCommand(edge, "file:///C:/x.html"); got != `"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe" --single-argument "file:///C:/x.html"` {
		t.Fatal(got)
	}
	ff := `"C:\Program Files\Mozilla Firefox\firefox.exe" -osint -url "%1"`
	if got := fillCommand(ff, "file:///C:/x.html"); got != `"C:\Program Files\Mozilla Firefox\firefox.exe" -osint -url "file:///C:/x.html"` {
		t.Fatal(got)
	}
	if got := fillCommand(`C:\b.exe`, "u"); got != `C:\b.exe "u"` {
		t.Fatal(got)
	}
	if exe, _ := splitCommand(`C:\b.exe -x`); exe != `C:\b.exe` {
		t.Fatal(exe)
	}
}
