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
