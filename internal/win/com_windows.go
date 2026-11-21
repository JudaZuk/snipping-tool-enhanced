// SPDX-License-Identifier: Apache-2.0

//go:build windows

package win

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// COM is a raw COM interface pointer. Methods are called by vtable index;
// the indices used in this project are taken from the Windows SDK metadata.
type COM struct{ vtbl *[128]uintptr }

// Call invokes vtable method i with the object as the first argument.
func (o *COM) Call(i int, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(o.vtbl[i], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return r
}

// HR calls method i and converts a failing HRESULT to an error.
func (o *COM) HR(name string, i int, args ...uintptr) error {
	if hr := int32(o.Call(i, args...)); hr < 0 {
		return fmt.Errorf("%s: HRESULT 0x%08X", name, uint32(hr))
	}
	return nil
}

// Release releases the interface (safe on nil).
func (o *COM) Release() {
	if o != nil {
		o.Call(2)
	}
}

// QueryInterface asks the object for another interface.
func (o *COM) QueryInterface(iid *windows.GUID) (*COM, error) {
	var out *COM
	if err := o.HR("QueryInterface", 0, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out))); err != nil {
		return nil, err
	}
	return out, nil
}

// AddRef and Release on a raw interface pointer.
func AddRef(p uintptr) {
	if p != 0 {
		(*COM)(PtrFromUintptr(p)).Call(1)
	}
}

// Release releases a raw interface pointer (safe on 0).
func Release(p uintptr) {
	if p != 0 {
		(*COM)(PtrFromUintptr(p)).Call(2)
	}
}

// HRErr converts an HRESULT returned by a plain DLL function.
func HRErr(name string, r uintptr) error {
	if int32(r) < 0 {
		return fmt.Errorf("%s: HRESULT 0x%08X", name, uint32(r))
	}
	return nil
}

// MustGUID parses "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx".
func MustGUID(s string) windows.GUID {
	g, err := windows.GUIDFromString("{" + s + "}")
	if err != nil {
		panic(err)
	}
	return g
}

var pSetWindowDisplayAffinity = user32.NewProc("SetWindowDisplayAffinity")

// ExcludeFromCapture keeps a window out of screenshots and recordings
// (WDA_EXCLUDEFROMCAPTURE, Windows 10 2004 and later).
func ExcludeFromCapture(h windows.HWND) {
	pSetWindowDisplayAffinity.Call(uintptr(h), 0x11)
}
