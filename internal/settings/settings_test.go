// SPDX-License-Identifier: Apache-2.0

package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsAndRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x", "settings.json")
	if Load(p) != Default() {
		t.Fatal("missing file must give defaults")
	}
	s := Default()
	s.MultipleWindows, s.Mode, s.Delay, s.BorderEnabled = true, "window", 5, true
	if err := Save(p, s); err != nil {
		t.Fatal(err)
	}
	if Load(p) != s {
		t.Fatalf("round trip: %+v", Load(p))
	}
}

func TestMigrateRecordingAutoSave(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	os.WriteFile(p, []byte(`{"autoSaveRecordings":true}`), 0o644) // 1.1.x file
	if Load(p).AutoSaveRecordings {
		t.Fatal("old files must migrate to no automatic recording save")
	}
	s := Default()
	s.AutoSaveRecordings = true // user turned it on in 1.2+
	Save(p, s)
	if !Load(p).AutoSaveRecordings || Default().AutoSaveRecordings {
		t.Fatal("explicit choice must be kept; default is off")
	}
}

func TestNormalize(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	os.WriteFile(p, []byte(`{"mode":"x","delay":7,"theme":"neon","borderThickness":99,"borderColor":"red","autoCopyChanges":false}`), 0o644)
	s := Load(p)
	if s.Mode != "rectangle" || s.Delay != 0 || s.Theme != "system" || s.BorderThickness != 20 || s.BorderColor != "#000000" || s.AutoCopyChanges {
		t.Fatalf("%+v", s)
	}
	os.WriteFile(p, []byte(`{broken`), 0o644)
	if Load(p) != Default() {
		t.Fatal("broken file must give defaults")
	}
	if _, err := FromJSON([]byte(`nope`)); err == nil {
		t.Fatal("want error")
	}
}
