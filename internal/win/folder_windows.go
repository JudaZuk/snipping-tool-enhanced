// SPDX-License-Identifier: Apache-2.0

//go:build windows

package win

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ole32             = windows.NewLazySystemDLL("ole32.dll")
	pCoCreateInstance = ole32.NewProc("CoCreateInstance")
	pCoTaskMemFree    = ole32.NewProc("CoTaskMemFree")

	clsidFileOpenDialog = windows.GUID{Data1: 0xDC1C5A9C, Data2: 0xE88A, Data3: 0x4DDE, Data4: [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidIFileOpenDialog  = windows.GUID{Data1: 0xD57C7288, Data2: 0xD4AD, Data3: 0x4768, Data4: [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
	iidIShellItem       = windows.GUID{Data1: 0x43826D1E, Data2: 0xE718, Data3: 0x42EE, Data4: [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}
)

type comObj struct{ vtbl *[64]uintptr }

func (o *comObj) call(i int, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(o.vtbl[i], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return r
}
func (o *comObj) release() { o.call(2) }

// PickFolder shows the modern folder picker (IFileOpenDialog with
// FOS_PICKFOLDERS). Returns "" if cancelled. COM must be initialized on the
// calling thread (WebView2 does this for the editor thread).
func PickFolder(owner windows.HWND, title, start string) string {
	var dlg *comObj
	if hr, _, _ := pCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidFileOpenDialog)), 0, 1, /*CLSCTX_INPROC_SERVER*/
		uintptr(unsafe.Pointer(&iidIFileOpenDialog)), uintptr(unsafe.Pointer(&dlg))); hr != 0 || dlg == nil {
		return ""
	}
	defer dlg.release()
	const fosPickFolders, fosForceFS = 0x20, 0x40
	var opts uint32
	dlg.call(10, uintptr(unsafe.Pointer(&opts))) // GetOptions
	dlg.call(9, uintptr(opts|fosPickFolders|fosForceFS))
	dlg.call(17, uintptr(unsafe.Pointer(Str(title)))) // SetTitle
	_ = start
	if hr := dlg.call(3, uintptr(owner)); hr != 0 { // Show
		return ""
	}
	var item *comObj
	if hr := dlg.call(20, uintptr(unsafe.Pointer(&item))); hr != 0 || item == nil { // GetResult
		return ""
	}
	defer item.release()
	var p *uint16
	const sigdnFileSysPath = 0x80058000
	if hr := item.call(5, sigdnFileSysPath, uintptr(unsafe.Pointer(&p))); hr != 0 || p == nil { // GetDisplayName
		return ""
	}
	defer pCoTaskMemFree.Call(uintptr(unsafe.Pointer(p)))
	return windows.UTF16PtrToString(p)
}

var _ = iidIShellItem
