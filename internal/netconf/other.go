//go:build !windows

package netconf

import "fmt"

var errUnsupported = fmt.Errorf("wireless configuration is only wired up for Windows")

func Scan() ([]Network, error)            { return nil, errUnsupported }
func Current() (Status, error)            { return Status{}, errUnsupported }
func Connect(ssid, password string) error { return errUnsupported }
func Forget(ssid string) error            { return errUnsupported }
