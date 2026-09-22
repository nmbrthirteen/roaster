package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAddsKioskPath(t *testing.T) {
	cases := map[string]string{
		`{"kioskUrl":"http://localhost:3000"}`:        "http://localhost:3000/kiosk",
		`{"kioskUrl":"http://localhost:3000/"}`:       "http://localhost:3000/kiosk",
		`{"kioskUrl":"http://localhost:3000/kiosk"}`:  "http://localhost:3000/kiosk",
		`{"kioskUrl":"https://roast.upgaming.com/k"}`: "https://roast.upgaming.com/k",
	}

	for body, want := range cases {
		path := filepath.Join(t.TempDir(), "roaster.json")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		if got.KioskURL != want {
			t.Errorf("%s\n got %q\nwant %q", body, got.KioskURL, want)
		}
	}
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("a missing settings file must not be an error: %v", err)
	}
	if got.KioskURL != Defaults().KioskURL || got.Provider != "demo" {
		t.Errorf("got %+v, want the defaults", got)
	}
}

func TestTheStandKeepsTbilisiTimeByDefault(t *testing.T) {
	zone := Defaults().Zone()
	if zone.String() != "Asia/Tbilisi" {
		t.Fatalf("zone %q, want Asia/Tbilisi", zone)
	}
	if _, off := time.Date(2026, 9, 22, 12, 0, 0, 0, zone).Zone(); off != 4*3600 {
		t.Errorf("Tbilisi should be UTC+4, got %d seconds", off)
	}
	if got := (Config{Timezone: "Europe/Berlin"}).Zone().String(); got != "Europe/Berlin" {
		t.Errorf("a configured zone should win, got %q", got)
	}
}
