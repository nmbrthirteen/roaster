package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v1.0.0", "v1.0.1", true},
		{"v1.2.3", "v1.2.3", false},
		{"v1.2.3", "v1.2.2", false},
		{"v1.2.3", "v1.3.0", true},
		{"v1.9.9", "v2.0.0", true},
		{"dev", "v0.0.1", true},
		{"v1.0.0", "not-a-version", false},
		{"garbage", "v1.0.0", true},
		{"garbage", "also garbage", false},
	}
	for _, c := range cases {
		if got := Newer(c.current, c.latest); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

// server fakes a GitHub release with two binaries and a SHA256SUMS asset, so
// Check and Apply can be exercised end to end without touching the network.
type server struct {
	*httptest.Server
	kioskBytes   []byte
	roasterBytes []byte
}

func newServer(t *testing.T, kioskBytes, roasterBytes []byte) *server {
	t.Helper()
	arch := runtime.GOARCH
	kioskName := "kiosk-windows-" + arch + ".exe"
	roasterName := "roaster-windows-" + arch + ".exe"

	mux := http.NewServeMux()
	s := &server{kioskBytes: kioskBytes, roasterBytes: roasterBytes}
	srv := httptest.NewServer(mux)
	s.Server = srv

	sum := func(b []byte) string {
		h := sha256.Sum256(b)
		return hex.EncodeToString(h[:])
	}
	sums := fmt.Sprintf("%s  %s\n%s  %s\n", sum(kioskBytes), kioskName, sum(roasterBytes), roasterName)

	mux.HandleFunc("/repos/nmbrthirteen/roaster/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name":"v9.9.9","assets":[
			{"name":%q,"browser_download_url":%q,"size":%d},
			{"name":%q,"browser_download_url":%q,"size":%d},
			{"name":"SHA256SUMS","browser_download_url":%q,"size":%d}
		]}`,
			kioskName, srv.URL+"/assets/"+kioskName, len(kioskBytes),
			roasterName, srv.URL+"/assets/"+roasterName, len(roasterBytes),
			srv.URL+"/assets/SHA256SUMS", len(sums))
	})
	mux.HandleFunc("/assets/"+kioskName, func(w http.ResponseWriter, r *http.Request) { w.Write(kioskBytes) })
	mux.HandleFunc("/assets/"+roasterName, func(w http.ResponseWriter, r *http.Request) { w.Write(roasterBytes) })
	mux.HandleFunc("/assets/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, sums) })

	t.Cleanup(srv.Close)
	return s
}

func withFakeAPI(t *testing.T, url string) {
	t.Helper()
	prev := apiBase
	apiBase = url
	t.Cleanup(func() { apiBase = prev })
}

func withWindows(t *testing.T) {
	t.Helper()
	prev := goos
	goos = "windows"
	t.Cleanup(func() { goos = prev })
}

func TestApplySkipsAnUnchangedBinaryAndSwapsTheOther(t *testing.T) {
	withWindows(t)
	kioskBytes := []byte("kiosk build one")
	roasterBytes := []byte("roaster build two")
	srv := newServer(t, kioskBytes, []byte("roaster build two, newer"))
	withFakeAPI(t, srv.URL)

	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "kiosk.exe"), kioskBytes)     // identical to the release
	mustWrite(t, filepath.Join(dir, "roaster.exe"), roasterBytes) // the release differs

	rel, err := Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !rel.Newer || rel.Tag != "v9.9.9" {
		t.Fatalf("Check: got %+v", rel)
	}

	res, err := Apply(context.Background(), rel, dir)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Version != "v9.9.9" {
		t.Errorf("Result.Version = %q, want v9.9.9", res.Version)
	}
	if res.NeedsReboot {
		t.Errorf("kiosk.exe was identical; NeedsReboot should be false")
	}

	if got := mustRead(t, filepath.Join(dir, "kiosk.exe")); string(got) != string(kioskBytes) {
		t.Errorf("kiosk.exe should be untouched, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "kiosk.exe.old")); err == nil {
		t.Errorf("kiosk.exe.old should not exist; kiosk.exe was never swapped")
	}

	if got := mustRead(t, filepath.Join(dir, "roaster.exe")); string(got) != "roaster build two, newer" {
		t.Errorf("roaster.exe should be the new build, got %q", got)
	}
	if got := mustRead(t, filepath.Join(dir, "roaster.exe.old")); string(got) != string(roasterBytes) {
		t.Errorf("roaster.exe.old should hold the previous build, got %q", got)
	}

	if !Pending(dir) {
		t.Errorf("Pending should be true right after Apply")
	}
}

func TestApplyLeavesFilesUntouchedOnChecksumMismatch(t *testing.T) {
	withWindows(t)
	kioskBytes := []byte("kiosk original")
	roasterBytes := []byte("roaster original")
	srv := newServer(t, kioskBytes, roasterBytes)
	withFakeAPI(t, srv.URL)

	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "kiosk.exe"), kioskBytes)
	mustWrite(t, filepath.Join(dir, "roaster.exe"), roasterBytes)

	rel, err := Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	// Corrupt the URL for one asset so its bytes no longer match SHA256SUMS.
	rel.KioskURL = srv.URL + "/assets/SHA256SUMS"

	if _, err := Apply(context.Background(), rel, dir); err == nil {
		t.Fatal("Apply should have failed the checksum")
	}

	if got := mustRead(t, filepath.Join(dir, "kiosk.exe")); string(got) != string(kioskBytes) {
		t.Errorf("kiosk.exe should be untouched, got %q", got)
	}
	if got := mustRead(t, filepath.Join(dir, "roaster.exe")); string(got) != string(roasterBytes) {
		t.Errorf("roaster.exe should be untouched, got %q", got)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".new" {
			t.Errorf("no .new file should be left behind, found %s", e.Name())
		}
	}
	if Pending(dir) {
		t.Errorf("a failed Apply should not leave a pending marker")
	}
}

func TestRollbackRestoresTheOldBytes(t *testing.T) {
	withWindows(t)
	kioskBytes := []byte("kiosk original")
	roasterBytes := []byte("roaster original")
	srv := newServer(t, kioskBytes, []byte("roaster newer"))
	withFakeAPI(t, srv.URL)

	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "kiosk.exe"), kioskBytes)
	mustWrite(t, filepath.Join(dir, "roaster.exe"), roasterBytes)

	rel, err := Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if _, err := Apply(context.Background(), rel, dir); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if err := Rollback(dir); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	if got := mustRead(t, filepath.Join(dir, "roaster.exe")); string(got) != string(roasterBytes) {
		t.Errorf("roaster.exe should be back to the original, got %q", got)
	}
	if got := mustRead(t, filepath.Join(dir, "roaster.exe.bad")); string(got) != "roaster newer" {
		t.Errorf("the rolled-back build should be kept as .bad, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "roaster.exe.old")); err == nil {
		t.Errorf("roaster.exe.old should be gone after rollback")
	}
	if Pending(dir) {
		t.Errorf("Rollback should clear the pending marker")
	}
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
