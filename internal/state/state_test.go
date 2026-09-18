package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveKeepsAPathItCanWriteBackTo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.WriteFile("roaster.json", []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Resolve("roaster.json"); got != "roaster.json" {
		t.Errorf("a settings file in a writable folder should be used where it is, got %q", got)
	}

	if err := os.Mkdir("events", 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Resolve("events"); got != "events" {
		t.Errorf("a writable events folder should be used where it is, got %q", got)
	}

	if got, want := Resolve("missing.json"), Path("missing.json"); got != want {
		t.Errorf("a file that is not there should come from the state folder\n got %q\nwant %q", got, want)
	}

	abs := filepath.Join(dir, "roaster.json")
	if got := Resolve(abs); got != abs {
		t.Errorf("an absolute path should be left alone, got %q", got)
	}
}

func TestWritableAnswersByWriting(t *testing.T) {
	dir := t.TempDir()
	if !Writable(dir) {
		t.Fatalf("a fresh temporary folder should be writable")
	}

	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	if os.Getuid() == 0 {
		t.Skip("root writes anywhere, which is not what a packaged app does")
	}
	if Writable(locked) {
		t.Errorf("a read-only folder should not be writable")
	}
	if Writable(filepath.Join(dir, "missing")) {
		t.Errorf("a folder that is not there should not be writable")
	}
}
