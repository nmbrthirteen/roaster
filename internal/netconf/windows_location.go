//go:build windows

package netconf

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

func dword(path, name string) (uint64, bool) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
	if err != nil {
		return 0, false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue(name)
	return v, err == nil
}

func text(path, name string) string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, _ := k.GetStringValue(name)
	return v
}

func locationBlocker() string {
	if v, ok := dword(`SOFTWARE\Policies\Microsoft\Windows\LocationAndSensors`, "DisableLocation"); ok && v == 1 {
		return "a Windows policy turns location off"
	}
	if v, ok := dword(`SOFTWARE\Policies\Microsoft\Windows\AppPrivacy`, "LetAppsAccessLocation"); ok && v == 2 {
		return "a Windows policy denies apps location"
	}
	if v, ok := dword(`SYSTEM\CurrentControlSet\Services\lfsvc`, "Start"); ok && v == 4 {
		return "the Windows location service is disabled"
	}
	if text(`SOFTWARE\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\location`, "Value") == "Deny" {
		return "location is switched off for this device"
	}
	return "location is off"
}

func locationError() error {
	return fmt.Errorf("Windows lists Wi-Fi networks only with location on, and %s. Run scripts\\kioskmode.bat again as administrator to turn it on", locationBlocker())
}
