//go:build windows

package netconf

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	wlanapi            = syscall.NewLazyDLL("wlanapi.dll")
	wlanOpenHandle     = wlanapi.NewProc("WlanOpenHandle")
	wlanEnumInterfaces = wlanapi.NewProc("WlanEnumInterfaces")
	wlanScan           = wlanapi.NewProc("WlanScan")
	wlanFreeMemory     = wlanapi.NewProc("WlanFreeMemory")
	wlanCloseHandle    = wlanapi.NewProc("WlanCloseHandle")
)

type wlanInterfaceInfo struct {
	guid        [16]byte
	description [256]uint16
	state       uint32
}

type wlanInterfaceList struct {
	count uint32
	index uint32
	items [64]wlanInterfaceInfo
}

const scanWait = 4 * time.Second

func rescan() {
	if wlanapi.Load() != nil {
		return
	}
	var version uint32
	var handle syscall.Handle
	if r, _, _ := wlanOpenHandle.Call(2, 0, uintptr(unsafe.Pointer(&version)), uintptr(unsafe.Pointer(&handle))); r != 0 {
		return
	}
	defer wlanCloseHandle.Call(uintptr(handle), 0)

	var list *wlanInterfaceList
	if r, _, _ := wlanEnumInterfaces.Call(uintptr(handle), 0, uintptr(unsafe.Pointer(&list))); r != 0 || list == nil {
		return
	}
	defer wlanFreeMemory.Call(uintptr(unsafe.Pointer(list)))

	n := min(int(list.count), len(list.items))
	for i := range n {
		wlanScan.Call(uintptr(handle), uintptr(unsafe.Pointer(&list.items[i].guid)), 0, 0, 0)
	}
	if n > 0 {
		time.Sleep(scanWait)
	}
}

var wlanGetAvailableNetworkList = wlanapi.NewProc("WlanGetAvailableNetworkList")

type dot11SSID struct {
	length uint32
	bytes  [32]byte
}

type wlanAvailableNetwork struct {
	profileName   [256]uint16
	ssid          dot11SSID
	bssType       uint32
	bssids        uint32
	connectable   int32
	notConnReason uint32
	phyTypes      uint32
	phyTypeList   [8]uint32
	morePhyTypes  int32
	signal        uint32
	secured       int32
	auth          uint32
	cipher        uint32
	flags         uint32
	reserved      uint32
}

const (
	networkConnected  = 1
	networkHasProfile = 2
)

// netsh writes network names in the console's code page, which turns the
// curly apostrophe of every "Name’s iPhone" hotspot into a straight one, and
// then no network by that name exists. The Wi-Fi API hands out the bytes.
func available() ([]found, error) {
	if err := wlanapi.Load(); err != nil {
		return nil, err
	}
	var version uint32
	var handle syscall.Handle
	if r, _, _ := wlanOpenHandle.Call(2, 0, uintptr(unsafe.Pointer(&version)), uintptr(unsafe.Pointer(&handle))); r != 0 {
		return nil, syscall.Errno(r)
	}
	defer wlanCloseHandle.Call(uintptr(handle), 0)

	var ifaces *wlanInterfaceList
	if r, _, _ := wlanEnumInterfaces.Call(uintptr(handle), 0, uintptr(unsafe.Pointer(&ifaces))); r != 0 || ifaces == nil {
		return nil, syscall.Errno(r)
	}
	defer wlanFreeMemory.Call(uintptr(unsafe.Pointer(ifaces)))

	byName := map[string]int{}
	var list []found
	for i := range min(int(ifaces.count), len(ifaces.items)) {
		var nets *struct {
			count uint32
			index uint32
		}
		if r, _, _ := wlanGetAvailableNetworkList.Call(uintptr(handle), uintptr(unsafe.Pointer(&ifaces.items[i].guid)), 0, 0, uintptr(unsafe.Pointer(&nets))); r != 0 || nets == nil {
			return nil, syscall.Errno(r)
		}
		first := (*wlanAvailableNetwork)(unsafe.Add(unsafe.Pointer(nets), 8))
		for _, n := range unsafe.Slice(first, nets.count) {
			name := string(n.ssid.bytes[:min(n.ssid.length, 32)])
			if name == "" {
				continue
			}
			f := found{
				Network: Network{
					SSID:   name,
					Signal: int(n.signal),
					Secure: n.secured != 0,
					Saved:  n.flags&networkHasProfile != 0,
					Active: n.flags&networkConnected != 0,
				},
				auth:   authName(n.auth),
				cipher: cipherName(n.cipher),
			}
			// A network with a saved profile is listed once with it and once
			// without. Keep one, holding everything either entry knew.
			if at, ok := byName[name]; ok {
				had := &list[at]
				had.Saved = had.Saved || f.Saved
				had.Active = had.Active || f.Active
				had.Signal = max(had.Signal, f.Signal)
				continue
			}
			byName[name] = len(list)
			list = append(list, f)
		}
		wlanFreeMemory.Call(uintptr(unsafe.Pointer(nets)))
	}
	return list, nil
}

// authName and cipherName put the API's numbers in netsh's words, which is
// what profileSecurity reads.
func authName(algo uint32) string {
	switch algo {
	case 1:
		return "Open"
	case 3:
		return "WPA-Enterprise"
	case 4:
		return "WPA-Personal"
	case 6:
		return "WPA2-Enterprise"
	case 7:
		return "WPA2-Personal"
	case 8, 11:
		return "WPA3-Enterprise"
	case 9:
		return "WPA3-Personal"
	case 10:
		return "OWE"
	}
	return ""
}

func cipherName(algo uint32) string {
	switch algo {
	case 0:
		return "None"
	case 2:
		return "TKIP"
	}
	return "CCMP"
}
