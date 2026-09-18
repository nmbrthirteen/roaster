//go:build windows

package secret

import (
	"syscall"
	"unsafe"
)

// Windows DPAPI ties the ciphertext to this account on this machine, so copying
// the file to another computer yields nothing. It does not defend against
// someone who boots the kiosk account, which auto-login hands them, so it is a
// speed bump rather than a lock.
var (
	crypt32       = syscall.NewLazyDLL("crypt32.dll")
	kernel32      = syscall.NewLazyDLL("kernel32.dll")
	procProtect   = crypt32.NewProc("CryptProtectData")
	procUnprotect = crypt32.NewProc("CryptUnprotectData")
	procLocalFree = kernel32.NewProc("LocalFree")
)

type blob struct {
	cbData uint32
	pbData *byte
}

func call(proc *syscall.LazyProc, in []byte) ([]byte, error) {
	if len(in) == 0 {
		return nil, nil
	}
	src := blob{cbData: uint32(len(in)), pbData: &in[0]}
	var dst blob

	r, _, err := proc.Call(
		uintptr(unsafe.Pointer(&src)), 0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&dst)),
	)
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(dst.pbData)))

	out := make([]byte, dst.cbData)
	copy(out, unsafe.Slice(dst.pbData, dst.cbData))
	return out, nil
}

func protect(plain []byte) ([]byte, error)    { return call(procProtect, plain) }
func unprotect(sealed []byte) ([]byte, error) { return call(procUnprotect, sealed) }
