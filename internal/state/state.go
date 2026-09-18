// Package state finds the folder the app writes to.
//
// A folder copied onto a stand keeps settings, events, logs and the terminal
// token beside the binaries, which is what makes that folder the whole install.
// A packaged install cannot work that way: Windows mounts an MSIX read-only and
// a write beside the executable fails outright. So the folder is the
// executable's own when that is writable, and the user's local application data
// when it is not.
package state

import (
	"os"
	"path/filepath"
	"sync"
)

const folder = "Roaster"

var (
	once sync.Once
	dir  string
)

// Dir is where settings, events, logs and the terminal token live.
func Dir() string {
	once.Do(resolve)
	return dir
}

// Use pins the folder, for a tool that has to write into another install's
// state rather than work out its own. It only counts before anything has asked
// where the folder is.
func Use(path string) {
	if path == "" {
		return
	}
	once.Do(func() { dir = path })
}

// Path names a file in it.
func Path(name string) string { return filepath.Join(Dir(), name) }

func resolve() {
	if exe, err := os.Executable(); err == nil {
		if beside := filepath.Dir(exe); Writable(beside) {
			dir = beside
			return
		}
	}

	local, err := os.UserCacheDir() // %LocalAppData% on Windows
	if err != nil {
		dir = "."
		return
	}
	dir = filepath.Join(local, folder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		dir = "."
	}
}

// Writable answers by writing. Permissions do not say whether a packaged app is
// allowed to use a folder, and the answer decides where everything is kept.
func Writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".write-")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

// Resolve turns a relative path into the one to use: the working directory's
// copy when that can be written back to, and this folder's otherwise. Settings
// and event files are saved as well as read, and a copy inside a read-only
// package would fail the first time an operator picked a printer.
func Resolve(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	if info, err := os.Stat(path); err == nil {
		target := filepath.Dir(path)
		if info.IsDir() {
			target = path
		}
		if Writable(target) {
			return path
		}
	}
	return Path(path)
}

// Seed copies a file shipped beside the executable into the writable folder,
// once, so a package can carry the settings a device starts with. A file the
// operator has since changed is left alone.
func Seed(name string) {
	if name == "" || filepath.IsAbs(name) {
		return
	}
	target := Path(name)
	if _, err := os.Stat(target); err == nil {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(exe), name))
	if err != nil {
		return // nothing shipped, which is the ordinary case
	}
	os.WriteFile(target, raw, 0o644)
}

// SeedDir does the same for a folder of files, such as the event files a
// package carries.
func SeedDir(name string) {
	if name == "" || filepath.IsAbs(name) {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	source := filepath.Join(filepath.Dir(exe), name)
	entries, err := os.ReadDir(source)
	if err != nil {
		return
	}
	target := Path(name)
	if target == source {
		return
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(target, e.Name())); err == nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(source, e.Name()))
		if err != nil {
			continue
		}
		os.WriteFile(filepath.Join(target, e.Name()), raw, 0o644)
	}
}
