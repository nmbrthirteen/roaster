// Package update pulls the latest GitHub Release of the public repo and
// swaps it into the folder the running binaries live in, so a stand locked to
// kiosk mode can update itself from the hidden menu instead of an operator
// unlocking the device.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/upgaming/roaster/internal/state"
	"github.com/upgaming/roaster/internal/version"
)

// repo is the public repo a stand pulls builds from. Nothing here reads or
// writes anything private to it.
const repo = "nmbrthirteen/roaster"

// apiBase is a var so a test can point it at an httptest server instead of
// the real GitHub API.
var apiBase = "https://api.github.com"

// goos stands in for runtime.GOOS so a test on a non-Windows machine can still
// exercise Apply. Production never sets it.
var goos = runtime.GOOS

// markerFile records the version an update replaced, so the launcher knows a
// fresh install is on trial and can roll it back if it will not stay up.
const markerFile = "update.pending"

// maxAssetSize caps a single downloaded binary. A build of this app is a few
// megabytes; 200MB is headroom against a redirect gone wrong, not an
// expectation.
const maxAssetSize = 200 * 1024 * 1024

// maxSumsSize is generous for a text file of a handful of hash lines.
const maxSumsSize = 1024 * 1024

// mu keeps two updates from stepping on the same files at once. The hidden
// menu is the only caller, but a second tap while the first request is still
// in flight must not start a second download.
var mu sync.Mutex

// Release is what the latest GitHub Release offers this machine.
type Release struct {
	Tag        string // the release's tag, such as "v1.4.0"
	Newer      bool   // whether Tag is newer than version.Version
	KioskURL   string
	RoasterURL string
	SumsURL    string
}

// Result is what Apply did.
type Result struct {
	Version     string // the release now installed
	NeedsReboot bool   // kiosk.exe changed, so the window itself has to restart
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

// Check asks GitHub for the latest release and works out whether this machine
// can use it and whether it is newer than what is running.
func Check(ctx context.Context) (Release, error) {
	rel, err := fetchRelease(ctx)
	if err != nil {
		return Release{}, err
	}

	byName := map[string]ghAsset{}
	for _, a := range rel.Assets {
		byName[a.Name] = a
	}

	arch := runtime.GOARCH
	kiosk, ok1 := byName["kiosk-windows-"+arch+".exe"]
	roaster, ok2 := byName["roaster-windows-"+arch+".exe"]
	sums, ok3 := byName["SHA256SUMS"]
	if !ok1 || !ok2 || !ok3 {
		return Release{}, fmt.Errorf("release %s has no build for windows/%s", rel.TagName, arch)
	}

	return Release{
		Tag:        rel.TagName,
		Newer:      Newer(version.Version, rel.TagName),
		KioskURL:   kiosk.BrowserDownloadURL,
		RoasterURL: roaster.BrowserDownloadURL,
		SumsURL:    sums.BrowserDownloadURL,
	}, nil
}

func fetchRelease(ctx context.Context) (ghRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		apiBase+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return ghRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "roaster-kiosk-update")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ghRelease{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return ghRelease{}, fmt.Errorf("github: %s", res.Status)
	}

	var rel ghRelease
	if err := json.NewDecoder(res.Body).Decode(&rel); err != nil {
		return ghRelease{}, fmt.Errorf("github: %w", err)
	}
	return rel, nil
}

// semver is the three numbers a tag like "v1.2.3" carries. Nothing here reads
// pre-release or build suffixes; a stand only ever compares releases this
// project cuts itself.
type semver struct{ major, minor, patch int }

func parseSemver(tag string) (semver, bool) {
	tag = strings.TrimPrefix(tag, "v")
	parts := strings.SplitN(tag, ".", 3)
	if len(parts) != 3 {
		return semver{}, false
	}
	patch := parts[2]
	if i := strings.IndexAny(patch, "-+"); i >= 0 {
		patch = patch[:i]
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	p, err3 := strconv.Atoi(patch)
	if err1 != nil || err2 != nil || err3 != nil {
		return semver{}, false
	}
	return semver{major, minor, p}, true
}

// Newer says whether latest is a later release than current. An unparsable
// current, "dev" included, counts as older than any real release: a build
// made straight from source has no version of its own to defend. An
// unparsable latest is never newer, so a malformed tag cannot trigger an
// update.
func Newer(current, latest string) bool {
	lv, ok := parseSemver(latest)
	if !ok {
		return false
	}
	cv, ok := parseSemver(current)
	if !ok {
		return true
	}
	if lv.major != cv.major {
		return lv.major > cv.major
	}
	if lv.minor != cv.minor {
		return lv.minor > cv.minor
	}
	return lv.patch > cv.patch
}

type binary struct {
	installed string // the name it runs under, e.g. "kiosk.exe"
	asset     string // the name it ships under in the release
	url       string
}

// Apply downloads rel into dir, the folder the running binaries live in, and
// swaps them in. It refuses outright on anything that is not Windows or on a
// folder it cannot write to, and it leaves the running install untouched on
// any failure after that: a partial update on a locked stand is worse than no
// update.
func Apply(ctx context.Context, rel Release, dir string) (Result, error) {
	if goos != "windows" {
		return Result{}, errors.New("Updates are only built for Windows.")
	}
	if !state.Writable(dir) {
		return Result{}, errors.New("The install folder is read-only for this account. Run scripts\\kioskmode.bat again as administrator.")
	}

	mu.Lock()
	defer mu.Unlock()

	sumBytes, err := fetchBytes(ctx, rel.SumsURL, maxSumsSize)
	if err != nil {
		return Result{}, fmt.Errorf("checksums: %w", err)
	}
	sums := parseSums(sumBytes)

	arch := runtime.GOARCH
	binaries := []binary{
		{installed: "kiosk.exe", asset: "kiosk-windows-" + arch + ".exe", url: rel.KioskURL},
		{installed: "roaster.exe", asset: "roaster-windows-" + arch + ".exe", url: rel.RoasterURL},
	}

	hashes := make(map[string]string, len(binaries))
	for _, b := range binaries {
		want, ok := sums[b.asset]
		if !ok {
			return Result{}, fmt.Errorf("SHA256SUMS has no entry for %s", b.asset)
		}
		dest := filepath.Join(dir, b.installed+".new")
		got, err := downloadAsset(ctx, b.url, dest)
		if err != nil {
			return Result{}, err
		}
		if !strings.EqualFold(got, want) {
			os.Remove(dest)
			return Result{}, fmt.Errorf("%s failed its checksum; nothing was installed", b.installed)
		}
		hashes[b.installed] = got
	}

	needsReboot := false
	for _, b := range binaries {
		newPath := filepath.Join(dir, b.installed+".new")
		installedPath := filepath.Join(dir, b.installed)

		if sameHash(installedPath, hashes[b.installed]) {
			os.Remove(newPath)
			continue
		}

		oldPath := installedPath + ".old"
		if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
			return Result{}, errors.New("An earlier update is still running. Reboot the device, then update again.")
		}
		// Renaming a running exe works on Windows; the file stays open under its
		// old name until whatever is running it exits.
		if err := os.Rename(installedPath, oldPath); err != nil {
			return Result{}, fmt.Errorf("could not step %s aside: %w", b.installed, err)
		}
		if err := os.Rename(newPath, installedPath); err != nil {
			return Result{}, fmt.Errorf("could not install %s: %w", b.installed, err)
		}
		if b.installed == "kiosk.exe" {
			needsReboot = true
		}
	}

	if err := os.WriteFile(filepath.Join(dir, markerFile), []byte(version.Version), 0o644); err != nil {
		return Result{}, fmt.Errorf("could not record the update: %w", err)
	}

	return Result{Version: rel.Tag, NeedsReboot: needsReboot}, nil
}

// Rollback puts back whatever an update stepped aside. It is what the
// launcher calls on itself when the build it just installed will not stay up.
func Rollback(dir string) error {
	for _, name := range []string{"kiosk.exe", "roaster.exe"} {
		oldPath := filepath.Join(dir, name+".old")
		if _, err := os.Stat(oldPath); err != nil {
			continue
		}
		installedPath := filepath.Join(dir, name)
		badPath := installedPath + ".bad"

		if err := os.Remove(badPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("could not clear %s: %w", filepath.Base(badPath), err)
		}
		if err := os.Rename(installedPath, badPath); err != nil {
			return fmt.Errorf("could not set %s aside: %w", name, err)
		}
		if err := os.Rename(oldPath, installedPath); err != nil {
			return fmt.Errorf("could not restore %s: %w", name, err)
		}
	}
	os.Remove(filepath.Join(dir, markerFile))
	return nil
}

// Pending says an update in dir has not proven itself yet.
func Pending(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, markerFile))
	return err == nil
}

// Settle marks the update in dir as good: it ran long enough that the
// launcher trusts it and will not roll it back.
func Settle(dir string) {
	os.Remove(filepath.Join(dir, markerFile))
}

func sameHash(path, want string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false
	}
	return hex.EncodeToString(h.Sum(nil)) == want
}

func fetchBytes(ctx context.Context, url string, cap int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, res.Status)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, cap+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > cap {
		return nil, fmt.Errorf("%s is larger than expected", url)
	}
	return data, nil
}

// downloadAsset streams url into dest while hashing it, so a 200MB file is
// never held twice over in memory. dest is removed on any failure, including
// the file coming in over the cap.
func downloadAsset(ctx context.Context, url, dest string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: %s", filepath.Base(dest), res.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(res.Body, maxAssetSize+1))
	closeErr := f.Close()

	if copyErr != nil {
		os.Remove(dest)
		return "", copyErr
	}
	if closeErr != nil {
		os.Remove(dest)
		return "", closeErr
	}
	if n > maxAssetSize {
		os.Remove(dest)
		return "", fmt.Errorf("%s is larger than the 200MB update limit", filepath.Base(dest))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// parseSums reads sha256sum's own format: a hex digest, two spaces (or one,
// or a binary-mode "*"), then the filename.
func parseSums(data []byte) map[string]string {
	sums := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		sums[strings.TrimPrefix(fields[1], "*")] = fields[0]
	}
	return sums
}
