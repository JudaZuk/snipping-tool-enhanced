// SPDX-License-Identifier: Apache-2.0

//go:build windows

// Package share opens the Windows share sheet (Mail, Teams, Nearby sharing,
// …) for a file, through the DataTransferManager interop that desktop apps
// use (white paper §6: "Share via the Windows share sheet").
package share

import (
	"errors"
	"fmt"
	"image"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"snippingtoolenhanced/internal/win"
)

var (
	combase                 = windows.NewLazySystemDLL("combase.dll")
	pRoGetActivationFactory = combase.NewProc("RoGetActivationFactory")
	pWindowsCreateString    = combase.NewProc("WindowsCreateString")
	pWindowsDeleteString    = combase.NewProc("WindowsDeleteString")
)

// Interface IDs (Windows SDK metadata). Parameterized ones are computed from
// their WinRT type signatures.
var (
	iidUnknown                  = win.MustGUID("00000000-0000-0000-c000-000000000046")
	iidInspectable              = win.MustGUID("af86e2e0-b12d-4c6a-9c5a-d7aa65101e90")
	iidAgileObject              = win.MustGUID("94ea2b94-e9cc-49e0-c0ff-ee64ca8f5b90")
	iidAsyncInfo                = win.MustGUID("00000036-0000-0000-c000-000000000046")
	iidDataTransferManagerInter = win.MustGUID("3a3dcd6c-3eab-43dc-bcde-45671ce800c8")
	iidDataTransferManager      = win.MustGUID("a5caee9b-8708-49d1-8d36-67d25a8da00c")
	iidStorageFileStatics       = win.MustGUID("5984c710-daf2-43c8-8bb4-a4d3eacfd03f")
	iidStorageItem              = win.MustGUID("4207a996-ca2f-42f7-bde8-8b10457a7f30")
	// TypedEventHandler<DataTransferManager, DataRequestedEventArgs>
	iidDataRequestedHandler = win.MustGUID("ec6f9cc8-46d0-5e0e-b4d2-7d7773ae37a0")
	// IIterable<IStorageItem>, IIterator<IStorageItem>
	iidIterableStorageItem = win.MustGUID("bb8b8418-65d1-544b-b083-6d172f568c73")
	iidIteratorStorageItem = win.MustGUID("05b487c2-3830-5d3c-98da-25fa11542dbd")
)

// Vtable indices.
const (
	interopGetForWindow          = 3 // IDataTransferManagerInterop
	interopShowShareUIForWindow  = 4
	dtmAddDataRequested          = 6 // IDataTransferManager
	argsGetRequest               = 6 // IDataRequestedEventArgs
	requestGetData               = 6 // IDataRequest
	requestFailWithDisplayText   = 9
	packageGetProperties         = 7 // IDataPackage
	packageSetStorageItemsRO     = 22
	propsPutTitle                = 7 // IDataPackagePropertySet
	propsPutApplicationName      = 14
	fileStaticsGetFileFromPath   = 6 // IStorageFileStatics
	asyncInfoStatus              = 7 // IAsyncInfo
	asyncOpGetResults            = 8 // IAsyncOperation<StorageFile>
	asyncStarted, asyncCompleted = 0, 1
	eNoInterface                 = 0x80004002
	eBounds                      = 0x8000000B
)

func hstring(s string) (uintptr, error) {
	u, err := windows.UTF16FromString(s)
	if err != nil {
		return 0, err
	}
	var h uintptr
	r, _, _ := pWindowsCreateString.Call(uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&h)))
	return h, win.HRErr("WindowsCreateString", r)
}

func deleteHString(h uintptr) { pWindowsDeleteString.Call(h) }

func factory(class string, iid *windows.GUID) (*win.COM, error) {
	h, err := hstring(class)
	if err != nil {
		return nil, err
	}
	defer deleteHString(h)
	var f *win.COM
	r, _, _ := pRoGetActivationFactory.Call(h, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&f)))
	if err := win.HRErr("RoGetActivationFactory "+class, r); err != nil {
		return nil, err
	}
	return f, nil
}

// pending is what each window shares next; the DataRequested handler reads it.
type pending struct {
	item  uintptr // IStorageItem (AddRef'd)
	title string
}

var (
	mu         sync.Mutex
	registered = map[windows.HWND]bool{}
	items      = map[windows.HWND]pending{}
	keep       []any // COM objects handed to Windows; never freed
)

// File shows the share sheet for the file at path, anchored to owner. It must
// run on owner's UI thread, which keeps pumping messages afterwards.
func File(owner windows.HWND, path string) error {
	_ = windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	item, err := storageItem(path)
	if err != nil {
		return err
	}
	ifac, err := factory("Windows.ApplicationModel.DataTransfer.DataTransferManager", &iidDataTransferManagerInter)
	if err != nil {
		win.Release(item)
		return err
	}
	defer ifac.Release()
	mu.Lock()
	if old, ok := items[owner]; ok && old.item != 0 {
		win.Release(old.item)
	}
	items[owner] = pending{item: item, title: filepath.Base(path)}
	first := !registered[owner]
	registered[owner] = true
	mu.Unlock()
	if first {
		var dtm *win.COM
		if err := ifac.HR("GetForWindow", interopGetForWindow, uintptr(owner), uintptr(unsafe.Pointer(&iidDataTransferManager)), uintptr(unsafe.Pointer(&dtm))); err != nil {
			mu.Lock()
			registered[owner] = false
			mu.Unlock()
			return err
		}
		var token int64
		h := newHandler(owner)
		err := dtm.HR("add_DataRequested", dtmAddDataRequested, uintptr(unsafe.Pointer(h)), uintptr(unsafe.Pointer(&token)))
		dtm.Release() // the registration keeps the manager alive for the window
		if err != nil {
			mu.Lock()
			registered[owner] = false
			mu.Unlock()
			return err
		}
	}
	return ifac.HR("ShowShareUIForWindow", interopShowShareUIForWindow, uintptr(owner))
}

// storageItem opens path as a StorageFile (an IStorageItem). The async call
// runs on a multithreaded-apartment thread and the result is agile.
func storageItem(path string) (uintptr, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return 0, err
	}
	type res struct {
		item uintptr
		err  error
	}
	ch := make(chan res, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		_ = windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED)
		item, err := openFile(abs)
		ch <- res{item, err}
	}()
	r := <-ch
	return r.item, r.err
}

func openFile(path string) (uintptr, error) {
	statics, err := factory("Windows.Storage.StorageFile", &iidStorageFileStatics)
	if err != nil {
		return 0, err
	}
	defer statics.Release()
	h, err := hstring(path)
	if err != nil {
		return 0, err
	}
	defer deleteHString(h)
	var op *win.COM
	if err := statics.HR("GetFileFromPathAsync", fileStaticsGetFileFromPath, h, uintptr(unsafe.Pointer(&op))); err != nil {
		return 0, err
	}
	defer op.Release()
	info, err := op.QueryInterface(&iidAsyncInfo)
	if err != nil {
		return 0, err
	}
	defer info.Release()
	deadline := time.Now().Add(15 * time.Second)
	for {
		var status int32
		if err := info.HR("get_Status", asyncInfoStatus, uintptr(unsafe.Pointer(&status))); err != nil {
			return 0, err
		}
		if status == asyncCompleted {
			break
		}
		if status != asyncStarted {
			return 0, fmt.Errorf("the file could not be opened for sharing (status %d)", status)
		}
		if time.Now().After(deadline) {
			return 0, errors.New("opening the file for sharing timed out")
		}
		time.Sleep(5 * time.Millisecond)
	}
	var file *win.COM
	if err := op.HR("GetResults", asyncOpGetResults, uintptr(unsafe.Pointer(&file))); err != nil {
		return 0, err
	}
	defer file.Release()
	item, err := file.QueryInterface(&iidStorageItem)
	if err != nil {
		return 0, err
	}
	return uintptr(unsafe.Pointer(item)), nil
}

// ---------- COM objects implemented in Go ----------

// handler is TypedEventHandler<DataTransferManager, DataRequestedEventArgs>.
type handler struct {
	vtbl  *[4]uintptr
	ref   int32
	owner windows.HWND
}

// iterable is IIterable<IStorageItem>; iterator is IIterator<IStorageItem>.
type iterable struct {
	vtbl  *[7]uintptr
	ref   int32
	items []uintptr
}

type iterator struct {
	vtbl  *[10]uintptr
	ref   int32
	items []uintptr
	pos   int
}

func guidAt(p uintptr) windows.GUID { return *(*windows.GUID)(win.PtrFromUintptr(p)) }

func setPtr(out, v uintptr) { *(*uintptr)(win.PtrFromUintptr(out)) = v }

// qi answers QueryInterface for an object that implements iid (plus IUnknown,
// IAgileObject and, for WinRT objects, IInspectable).
func qi(this, riid, out uintptr, ref *int32, iids ...windows.GUID) uintptr {
	g := guidAt(riid)
	ok := g == iidUnknown || g == iidAgileObject
	for _, i := range iids {
		ok = ok || g == i
	}
	if !ok {
		setPtr(out, 0)
		return eNoInterface
	}
	atomic.AddInt32(ref, 1)
	setPtr(out, this)
	return 0
}

var (
	handlerVtbl = &[4]uintptr{
		syscall.NewCallback(func(this, riid, out uintptr) uintptr {
			return qi(this, riid, out, &(*handler)(win.PtrFromUintptr(this)).ref, iidDataRequestedHandler)
		}),
		syscall.NewCallback(func(this uintptr) uintptr {
			return uintptr(atomic.AddInt32(&(*handler)(win.PtrFromUintptr(this)).ref, 1))
		}),
		syscall.NewCallback(func(this uintptr) uintptr {
			return uintptr(atomic.AddInt32(&(*handler)(win.PtrFromUintptr(this)).ref, -1))
		}),
		syscall.NewCallback(func(this, sender, args uintptr) uintptr {
			onDataRequested((*handler)(win.PtrFromUintptr(this)).owner, args)
			return 0
		}),
	}
	inspectableFns = [3]uintptr{
		syscall.NewCallback(func(this, count, iids uintptr) uintptr { // GetIids
			*(*uint32)(win.PtrFromUintptr(count)) = 0
			setPtr(iids, 0)
			return 0
		}),
		syscall.NewCallback(func(this, name uintptr) uintptr { setPtr(name, 0); return 0 }), // GetRuntimeClassName
		syscall.NewCallback(func(this, level uintptr) uintptr { // GetTrustLevel: BaseTrust
			*(*int32)(win.PtrFromUintptr(level)) = 0
			return 0
		}),
	}
	iterableVtbl = &[7]uintptr{
		syscall.NewCallback(func(this, riid, out uintptr) uintptr {
			return qi(this, riid, out, &(*iterable)(win.PtrFromUintptr(this)).ref, iidInspectable, iidIterableStorageItem)
		}),
		syscall.NewCallback(func(this uintptr) uintptr {
			return uintptr(atomic.AddInt32(&(*iterable)(win.PtrFromUintptr(this)).ref, 1))
		}),
		syscall.NewCallback(func(this uintptr) uintptr {
			return uintptr(atomic.AddInt32(&(*iterable)(win.PtrFromUintptr(this)).ref, -1))
		}),
		inspectableFns[0], inspectableFns[1], inspectableFns[2],
		syscall.NewCallback(func(this, out uintptr) uintptr { // First
			it := &iterator{vtbl: iteratorVtbl, ref: 1, items: (*iterable)(win.PtrFromUintptr(this)).items}
			mu.Lock()
			keep = append(keep, it)
			mu.Unlock()
			setPtr(out, uintptr(unsafe.Pointer(it)))
			return 0
		}),
	}
	iteratorVtbl = &[10]uintptr{
		syscall.NewCallback(func(this, riid, out uintptr) uintptr {
			return qi(this, riid, out, &(*iterator)(win.PtrFromUintptr(this)).ref, iidInspectable, iidIteratorStorageItem)
		}),
		syscall.NewCallback(func(this uintptr) uintptr {
			return uintptr(atomic.AddInt32(&(*iterator)(win.PtrFromUintptr(this)).ref, 1))
		}),
		syscall.NewCallback(func(this uintptr) uintptr {
			return uintptr(atomic.AddInt32(&(*iterator)(win.PtrFromUintptr(this)).ref, -1))
		}),
		inspectableFns[0], inspectableFns[1], inspectableFns[2],
		syscall.NewCallback(func(this, out uintptr) uintptr { // get_Current
			it := (*iterator)(win.PtrFromUintptr(this))
			if it.pos >= len(it.items) {
				setPtr(out, 0)
				return eBounds
			}
			win.AddRef(it.items[it.pos])
			setPtr(out, it.items[it.pos])
			return 0
		}),
		syscall.NewCallback(func(this, out uintptr) uintptr { // get_HasCurrent
			it := (*iterator)(win.PtrFromUintptr(this))
			*(*byte)(win.PtrFromUintptr(out)) = boolByte(it.pos < len(it.items))
			return 0
		}),
		syscall.NewCallback(func(this, out uintptr) uintptr { // MoveNext
			it := (*iterator)(win.PtrFromUintptr(this))
			if it.pos < len(it.items) {
				it.pos++
			}
			*(*byte)(win.PtrFromUintptr(out)) = boolByte(it.pos < len(it.items))
			return 0
		}),
		syscall.NewCallback(func(this, capacity, items, actual uintptr) uintptr { // GetMany
			it := (*iterator)(win.PtrFromUintptr(this))
			n := 0
			for n < int(uint32(capacity)) && it.pos < len(it.items) {
				win.AddRef(it.items[it.pos])
				setPtr(items+uintptr(n)*unsafe.Sizeof(uintptr(0)), it.items[it.pos])
				n++
				it.pos++
			}
			*(*uint32)(win.PtrFromUintptr(actual)) = uint32(n)
			return 0
		}),
	}
)

func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func newHandler(owner windows.HWND) *handler {
	h := &handler{vtbl: handlerVtbl, ref: 1, owner: owner}
	mu.Lock()
	keep = append(keep, h)
	mu.Unlock()
	return h
}

// onDataRequested fills the share request with the window's pending file.
func onDataRequested(owner windows.HWND, args uintptr) {
	a := (*win.COM)(win.PtrFromUintptr(args))
	var req *win.COM
	if a.HR("get_Request", argsGetRequest, uintptr(unsafe.Pointer(&req))) != nil || req == nil {
		return
	}
	defer req.Release()
	mu.Lock()
	p, ok := items[owner]
	mu.Unlock()
	if !ok || p.item == 0 {
		if h, err := hstring("There is nothing to share."); err == nil {
			req.Call(requestFailWithDisplayText, h)
			deleteHString(h)
		}
		return
	}
	var pkg *win.COM
	if req.HR("get_Data", requestGetData, uintptr(unsafe.Pointer(&pkg))) != nil || pkg == nil {
		return
	}
	defer pkg.Release()
	var props *win.COM
	if pkg.HR("get_Properties", packageGetProperties, uintptr(unsafe.Pointer(&props))) == nil && props != nil {
		if h, err := hstring(p.title); err == nil {
			props.Call(propsPutTitle, h) // a title is required
			deleteHString(h)
		}
		if h, err := hstring("Snipping Tool Enhanced"); err == nil {
			props.Call(propsPutApplicationName, h)
			deleteHString(h)
		}
		props.Release()
	}
	list := &iterable{vtbl: iterableVtbl, ref: 1, items: []uintptr{p.item}}
	mu.Lock()
	keep = append(keep, list)
	mu.Unlock()
	pkg.Call(packageSetStorageItemsRO, uintptr(unsafe.Pointer(list)))
}

// ---------- anchor window ----------

const anchorClass = "SnippingToolEnhancedShare"

var (
	anchorOnce sync.Once
	anchorHwnd windows.HWND
	anchorErr  error
	anchorProc = syscall.NewCallback(func(h, msg, wp, lp uintptr) uintptr {
		return win.DefWindowProc(windows.HWND(h), uint32(msg), wp, lp)
	})
)

// At shows the share sheet for path from a tiny invisible window at p
// (screen coordinates), for callers without a window of their own. Call it
// on a thread that keeps pumping messages; the window is reused.
func At(p image.Point, path string) error {
	anchorOnce.Do(func() {
		if anchorErr = win.RegisterClass(anchorClass, anchorProc, win.IDC_ARROW, 0); anchorErr != nil {
			return
		}
		anchorHwnd, anchorErr = win.CreateWindow(win.WS_EX_TOOLWINDOW|win.WS_EX_LAYERED|win.WS_EX_TOPMOST, anchorClass,
			"Snipping Tool Enhanced", win.WS_POPUP, int32(p.X), int32(p.Y), 1, 1)
		if anchorErr == nil {
			win.SetWindowAlpha(anchorHwnd, 0)
		}
	})
	if anchorErr != nil {
		return anchorErr
	}
	win.MoveWindow(anchorHwnd, int32(p.X), int32(p.Y), 1, 1)
	win.ShowWindow(anchorHwnd, win.SW_SHOW)
	win.SetForegroundWindow(anchorHwnd)
	return File(anchorHwnd, path)
}
