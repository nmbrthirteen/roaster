//go:build windows

package netconf

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	reSSID    = regexp.MustCompile(`(?m)^\s*SSID\s+\d+\s*:\s*(.*)$`)
	reSignal  = regexp.MustCompile(`(?m)^\s*Signal\s*:\s*(\d+)%`)
	reAuth    = regexp.MustCompile(`(?m)^\s*Authentication\s*:\s*(.*)$`)
	reProfile = regexp.MustCompile(`(?m)^\s*All User Profile\s*:\s*(.*)$`)
	reState   = regexp.MustCompile(`(?m)^\s*State\s*:\s*(.*)$`)
)

func netsh(args ...string) (string, error) {
	out, err := exec.Command("netsh", args...).CombinedOutput()
	return string(out), err
}

func Scan() ([]Network, error) {
	saved := map[string]bool{}
	if out, err := netsh("wlan", "show", "profiles"); err == nil {
		for _, m := range reProfile.FindAllStringSubmatch(out, -1) {
			saved[strings.TrimSpace(m[1])] = true
		}
	}

	rescan()
	out, err := netsh("wlan", "show", "networks", "mode=bssid")
	if strings.Contains(strings.ToLower(out), "location") {
		return nil, locationError()
	}
	if err != nil {
		return nil, fmt.Errorf("could not scan: %s", strings.TrimSpace(out))
	}

	var list []Network
	// Each network is a block starting at its SSID line.
	blocks := regexp.MustCompile(`(?m)^SSID \d+ :`).Split(out, -1)
	names := reSSID.FindAllStringSubmatch(out, -1)
	for i, block := range blocks[1:] {
		if i >= len(names) {
			break
		}
		name := strings.TrimSpace(names[i][1])
		if name == "" {
			continue
		}
		n := Network{SSID: name, Saved: saved[name]}
		if m := reSignal.FindStringSubmatch(block); m != nil {
			n.Signal, _ = strconv.Atoi(m[1])
		}
		if m := reAuth.FindStringSubmatch(block); m != nil {
			n.Secure = !strings.EqualFold(strings.TrimSpace(m[1]), "Open")
		}
		list = append(list, n)
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
	if password != "" {
		if err := addProfile(ssid, password); err != nil {
			return err
		}
	}
	out, err := netsh("wlan", "connect", "name="+ssid, "ssid="+ssid)
	if err != nil {
		return fmt.Errorf("could not connect: %s", strings.TrimSpace(out))
	}
	return nil
}

func Forget(ssid string) error {
	out, err := netsh("wlan", "delete", "profile", "name="+ssid)
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(out))
	}
	return nil
}

func addProfile(ssid, password string) error {
	f, err := tempProfile(ssid, password)
	if err != nil {
		return err
	}
	defer remove(f)

	if out, err := netsh("wlan", "add", "profile", "filename="+f, "user=all"); err != nil {
		return fmt.Errorf("could not save the network: %s", strings.TrimSpace(out))
	}
	return nil
}
