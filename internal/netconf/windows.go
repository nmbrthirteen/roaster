//go:build windows

package netconf

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	reSSID    = regexp.MustCompile(`(?m)^\s*SSID\s+\d+\s*:\s*(.*)$`)
	reSignal  = regexp.MustCompile(`(?m)^\s*Signal\s*:\s*(\d+)%`)
	reAuth    = regexp.MustCompile(`(?m)^\s*Authentication\s*:\s*(.*)$`)
	reProfile = regexp.MustCompile(`(?m)^\s*All User Profile\s*:\s*(.*)$`)
	reState   = regexp.MustCompile(`(?m)^\s*State\s*:\s*(.*)$`)
	reCipher  = regexp.MustCompile(`(?m)^\s*Encryption\s*:\s*(.*)$`)
	reBlock   = regexp.MustCompile(`(?m)^SSID \d+ :`)
)

type found struct {
	Network
	auth, cipher string
}

// parseNetworks reads netsh's network list. Each network is a block starting
// at its SSID line.
func parseNetworks(out string) []found {
	var list []found
	blocks := reBlock.Split(out, -1)
	names := reSSID.FindAllStringSubmatch(out, -1)
	for i, block := range blocks[1:] {
		if i >= len(names) {
			break
		}
		name := strings.TrimSpace(names[i][1])
		if name == "" {
			continue
		}
		f := found{Network: Network{SSID: name}}
		if m := reSignal.FindStringSubmatch(block); m != nil {
			f.Signal, _ = strconv.Atoi(m[1])
		}
		if m := reAuth.FindStringSubmatch(block); m != nil {
			f.auth = strings.TrimSpace(m[1])
			f.Secure = !strings.EqualFold(f.auth, "Open")
		}
		if m := reCipher.FindStringSubmatch(block); m != nil {
			f.cipher = strings.TrimSpace(m[1])
		}
		list = append(list, f)
	}
	return list
}

// security is how ssid asks to be joined, as the last scan saw it. A network
// out of sight is taken to be WPA2, the most common.
func security(ssid string) (auth, cipher string) {
	if f, ok := lookup(ssid); ok && f.auth != "" {
		return f.auth, f.cipher
	}
	out, _ := netsh("wlan", "show", "networks")
	for _, f := range parseNetworks(out) {
		if f.SSID == ssid {
			return f.auth, f.cipher
		}
	}
	return "WPA2-Personal", "CCMP"
}

func netsh(args ...string) (string, error) {
	out, err := exec.Command("netsh", args...).CombinedOutput()
	return string(out), err
}

func Scan() ([]Network, error) {
	rescan()
	if got, err := available(); err == nil && len(got) > 0 {
		list := make([]Network, 0, len(got))
		for _, f := range got {
			list = append(list, f.Network)
		}
		sort.Slice(list, func(a, b int) bool { return list[a].Signal > list[b].Signal })
		return list, nil
	}

	// netsh is what explains a scan Windows refused, such as location being
	// off, so it stays as the way through when the API comes back empty.
	saved := map[string]bool{}
	if out, err := netsh("wlan", "show", "profiles"); err == nil {
		for _, m := range reProfile.FindAllStringSubmatch(out, -1) {
			saved[strings.TrimSpace(m[1])] = true
		}
	}

	out, err := netsh("wlan", "show", "networks", "mode=bssid")
	if strings.Contains(strings.ToLower(out), "location") {
		return nil, locationError(strings.TrimSpace(out))
	}
	if err != nil {
		return nil, fmt.Errorf("could not scan: %s", strings.TrimSpace(out))
	}

	var list []Network
	for _, f := range parseNetworks(out) {
		f.Saved = saved[f.SSID]
		list = append(list, f.Network)
	}

	if now, err := Current(); err == nil && now.Connected {
		for i := range list {
			list[i].Active = list[i].SSID == now.SSID
		}
	}

	sort.Slice(list, func(a, b int) bool { return list[a].Signal > list[b].Signal })
	return list, nil
}

func Current() (Status, error) {
	out, err := netsh("wlan", "show", "interfaces")
	if err != nil {
		return Status{}, err
	}
	s := Status{}
	if m := reState.FindStringSubmatch(out); m != nil {
		s.Connected = strings.EqualFold(strings.TrimSpace(m[1]), "connected")
	}
	if m := regexp.MustCompile(`(?m)^\s*SSID\s*:\s*(.*)$`).FindStringSubmatch(out); m != nil {
		s.SSID = strings.TrimSpace(m[1])
	}
	if m := reSignal.FindStringSubmatch(out); m != nil {
		s.Signal, _ = strconv.Atoi(m[1])
	}
	return s, nil
}

// Connect joins a network, adding a WPA2 profile first when the password is
// given. Windows has no command that takes a password directly.
func Connect(ssid, password string) error {
	if ssid == "" {
		return fmt.Errorf("no network chosen")
	}
	if password != "" || !saved(ssid) {
		if err := addProfile(ssid, password); err != nil {
			return err
		}
	}
	out, err := netsh("wlan", "connect", "name="+ssid)
	if err != nil {
		return fmt.Errorf("could not connect: %s", strings.TrimSpace(out))
	}
	return joined(ssid)
}

// netsh reports success the moment it hands the request to Windows, which is
// before the network answers, so a wrong password looks like a success here.
func joined(ssid string) error {
	deadline := time.Now().Add(20 * time.Second)
	for {
		time.Sleep(time.Second)
		if f, ok := lookup(ssid); ok && f.Active {
			return nil
		}
		if now, err := Current(); err == nil && now.Connected && now.SSID == ssid {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s did not let this device on. The password is the usual reason", ssid)
		}
	}
}

// lookup finds ssid in what the Wi-Fi API sees right now.
func lookup(ssid string) (found, bool) {
	list, err := available()
	if err != nil {
		return found{}, false
	}
	for _, f := range list {
		if f.SSID == ssid {
			return f, true
		}
	}
	return found{}, false
}

func saved(ssid string) bool {
	if f, ok := lookup(ssid); ok {
		return f.Saved
	}
	out, err := netsh("wlan", "show", "profiles")
	if err != nil {
		return false
	}
	for _, m := range reProfile.FindAllStringSubmatch(out, -1) {
		if strings.TrimSpace(m[1]) == ssid {
			return true
		}
	}
	return false
}

func Forget(ssid string) error {
	out, err := netsh("wlan", "delete", "profile", "name="+ssid)
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(out))
	}
	return nil
}

func addProfile(ssid, password string) error {
	auth, cipher := security(ssid)
	f, err := tempProfile(ssid, password, auth, cipher)
	if err != nil {
		return err
	}
	defer remove(f)

	if out, err := netsh("wlan", "add", "profile", "filename="+f, "user=all"); err != nil {
		return fmt.Errorf("could not save the network: %s", strings.TrimSpace(out))
	}
	return nil
}
