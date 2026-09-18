//go:build !windows

package printer

import "fmt"

func openWindows(queue string) (Printer, error) {
	return nil, fmt.Errorf("win: printers need Windows; use lp:, usb: or tcp: here")
}
