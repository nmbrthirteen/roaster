//go:build windows

package main

import (
	"log"
	"os/exec"
)

// fullBrightness turns a built-in screen all the way up, so a stand reads in a
// bright hall. It goes through WMI because that is open to the standard account
// a kiosk signs in as, where the power plan is not. External monitors do not
// answer it, and nothing here needs them to.
func fullBrightness() {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"Get-CimInstance -Namespace root/WMI -ClassName WmiMonitorBrightnessMethods | "+
			"Invoke-CimMethod -MethodName WmiSetBrightness -Arguments @{Timeout=0; Brightness=100} | Out-Null")
	quiet(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("brightness: %v: %s", err, out)
	}
}
