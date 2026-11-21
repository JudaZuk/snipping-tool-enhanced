// SPDX-License-Identifier: Apache-2.0

package visualsearch

import (
	"strings"
	"testing"
)

func TestCommand(t *testing.T) {
	edge := `"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe" --single-argument %1`
	exe, _ := splitCommand(edge)
	if exe != `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe` {
		t.Fatal(exe)
	}
	if got := fillCommand(edge, "file:///C:/x.html"); got != `"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe" --single-argument file:///C:/x.html` {
		t.Fatal(got)
	}
	ff := `"C:\Program Files\Mozilla Firefox\firefox.exe" -osint -url "%1"`
	if got := fillCommand(ff, "file:///C:/x.html"); got != `"C:\Program Files\Mozilla Firefox\firefox.exe" -osint -url "file:///C:/x.html"` {
		t.Fatal(got)
	}
	if got := fillCommand(`C:\b.exe`, "u"); got != `C:\b.exe u` {
		t.Fatal(got)
	}
	// Chrome, the case that failed in 1.5.0: no quotes may be added.
	chrome := `"C:\Program Files\Google\Chrome\Application\chrome.exe" --single-argument %1`
	if got := fillCommand(chrome, "file:///C:/Users/peter/AppData/Local/x.html"); got != `"C:\Program Files\Google\Chrome\Application\chrome.exe" --single-argument file:///C:/Users/peter/AppData/Local/x.html` {
		t.Fatal(got)
	}
	if exe, _ := splitCommand(`C:\b.exe -x`); exe != `C:\b.exe` {
		t.Fatal(exe)
	}
	u := fileURL(`C:\Users\Peter Smith\AppData\Local\SnippingToolEnhanced\VisualSearch\search-1.html`)
	if u != "file:///C:/Users/Peter%20Smith/AppData/Local/SnippingToolEnhanced/VisualSearch/search-1.html" {
		t.Fatal(u)
	}
	if strings.ContainsAny(fileURL(`C:\a b\c#d%e.html`), " #") {
		t.Fatal("special characters must be escaped")
	}
}
