// SPDX-License-Identifier: Apache-2.0

//go:build windows

// Package win contains the minimal Win32 bindings used by Snipping Tool Enhanced.
// Everything is called through golang.org/x/sys/windows lazy DLLs, so the
// program builds without cgo.
package win

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")
	comdlg32 = windows.NewLazySystemDLL("comdlg32.dll")

	pRegisterClassExW          = user32.NewProc("RegisterClassExW")
	pCreateWindowExW           = user32.NewProc("CreateWindowExW")
	pDefWindowProcW            = user32.NewProc("DefWindowProcW")
	pDestroyWindow             = user32.NewProc("DestroyWindow")
	pShowWindow                = user32.NewProc("ShowWindow")
	pUpdateWindow              = user32.NewProc("UpdateWindow")
	pGetMessageW               = user32.NewProc("GetMessageW")
	pTranslateMessage          = user32.NewProc("TranslateMessage")
	pDispatchMessageW          = user32.NewProc("DispatchMessageW")
	pPostQuitMessage           = user32.NewProc("PostQuitMessage")
	pPostMessageW              = user32.NewProc("PostMessageW")
	pSendMessageW              = user32.NewProc("SendMessageW")
	pBeginPaint                = user32.NewProc("BeginPaint")
	pEndPaint                  = user32.NewProc("EndPaint")
	pInvalidateRect            = user32.NewProc("InvalidateRect")
	pGetDC                     = user32.NewProc("GetDC")
	pReleaseDC                 = user32.NewProc("ReleaseDC")
	pSetCapture                = user32.NewProc("SetCapture")
	pReleaseCapture            = user32.NewProc("ReleaseCapture")
	pLoadCursorW               = user32.NewProc("LoadCursorW")
	pSetCursor                 = user32.NewProc("SetCursor")
	pGetSystemMetrics          = user32.NewProc("GetSystemMetrics")
	pSetForegroundWindow       = user32.NewProc("SetForegroundWindow")
	pSetFocus                  = user32.NewProc("SetFocus")
	pGetKeyState               = user32.NewProc("GetKeyState")
	pEnumDisplayMonitors       = user32.NewProc("EnumDisplayMonitors")
	pGetMonitorInfoW           = user32.NewProc("GetMonitorInfoW")
	pRegisterHotKey            = user32.NewProc("RegisterHotKey")
	pUnregisterHotKey          = user32.NewProc("UnregisterHotKey")
	pCreatePopupMenu           = user32.NewProc("CreatePopupMenu")
	pAppendMenuW               = user32.NewProc("AppendMenuW")
	pTrackPopupMenu            = user32.NewProc("TrackPopupMenu")
	pDestroyMenu               = user32.NewProc("DestroyMenu")
	pGetCursorPos              = user32.NewProc("GetCursorPos")
	pSetTimer                  = user32.NewProc("SetTimer")
	pKillTimer                 = user32.NewProc("KillTimer")
	pFindWindowW               = user32.NewProc("FindWindowW")
	pMessageBoxW               = user32.NewProc("MessageBoxW")
	pRegisterWindowMessageW    = user32.NewProc("RegisterWindowMessageW")
	pGetCursorInfo             = user32.NewProc("GetCursorInfo")
	pGetIconInfo               = user32.NewProc("GetIconInfo")
	pDrawIconEx                = user32.NewProc("DrawIconEx")
	pCreateIconIndirect        = user32.NewProc("CreateIconIndirect")
	pFillRect                  = user32.NewProc("FillRect")
	pDrawTextW                 = user32.NewProc("DrawTextW")
	pIsWindowVisible           = user32.NewProc("IsWindowVisible")
	pIsIconic                  = user32.NewProc("IsIconic")
	pGetWindowTextW            = user32.NewProc("GetWindowTextW")
	pGetWindowLongPtrW         = user32.NewProc("GetWindowLongPtrW")
	pGetClassNameW             = user32.NewProc("GetClassNameW")
	pSetProcessDpiAwarenessCtx = user32.NewProc("SetProcessDpiAwarenessContext")
	pGetDpiForWindow           = user32.NewProc("GetDpiForWindow")
	pGetDpiForSystem           = user32.NewProc("GetDpiForSystem")
	pOpenClipboard             = user32.NewProc("OpenClipboard")
	pCloseClipboard            = user32.NewProc("CloseClipboard")
	pEmptyClipboard            = user32.NewProc("EmptyClipboard")
	pSetClipboardData          = user32.NewProc("SetClipboardData")
	pRegisterClipboardFormatW  = user32.NewProc("RegisterClipboardFormatW")
	pEnumWindows               = user32.NewProc("EnumWindows")
	pGetWindowThreadProcessId  = user32.NewProc("GetWindowThreadProcessId")
	pGetWindow                 = user32.NewProc("GetWindow")

	pCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	pCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	pSelectObject       = gdi32.NewProc("SelectObject")
	pDeleteObject       = gdi32.NewProc("DeleteObject")
	pDeleteDC           = gdi32.NewProc("DeleteDC")
	pBitBlt             = gdi32.NewProc("BitBlt")
	pCreateSolidBrush   = gdi32.NewProc("CreateSolidBrush")
	pSetBkMode          = gdi32.NewProc("SetBkMode")
	pSetTextColor       = gdi32.NewProc("SetTextColor")
	pCreateFontW        = gdi32.NewProc("CreateFontW")
	pCreateBitmap       = gdi32.NewProc("CreateBitmap")

	pGlobalAlloc      = kernel32.NewProc("GlobalAlloc")
	pGlobalLock       = kernel32.NewProc("GlobalLock")
	pGlobalUnlock     = kernel32.NewProc("GlobalUnlock")
	pGlobalFree       = kernel32.NewProc("GlobalFree")
	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	pShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	pShellExecuteW    = shell32.NewProc("ShellExecuteW")

	pDwmGetWindowAttribute = dwmapi.NewProc("DwmGetWindowAttribute")

	pGetSaveFileNameW = comdlg32.NewProc("GetSaveFileNameW")
	pGetOpenFileNameW = comdlg32.NewProc("GetOpenFileNameW")

	pCreatePen              = gdi32.NewProc("CreatePen")
	pPolyline               = gdi32.NewProc("Polyline")
	pCreatePolygonRgn       = gdi32.NewProc("CreatePolygonRgn")
	pSelectClipRgn          = gdi32.NewProc("SelectClipRgn")
	pGdiFlush               = gdi32.NewProc("GdiFlush")
	pSetWindowLongPtrW      = user32.NewProc("SetWindowLongPtrW")
	pCallWindowProcW        = user32.NewProc("CallWindowProcW")
	pAllowSetForegroundWnd  = user32.NewProc("AllowSetForegroundWindow")
	pSetWindowPos           = user32.NewProc("SetWindowPos")
	pDwmSetWindowAttribute  = dwmapi.NewProc("DwmSetWindowAttribute")
	pIsWindowVisibleExport  = user32.NewProc("IsWindowVisible")
	pBringWindowToTop       = user32.NewProc("BringWindowToTop")
	pSetLayeredWindowAttrib = user32.NewProc("SetLayeredWindowAttributes")
	pMoveWindow             = user32.NewProc("MoveWindow")
)

// ---- additional helpers ----

const (
	SW_RESTORE        = 9
	SW_SHOWNOACTIVATE = 4
	HWND_TOPMOST      = ^uintptr(0) // (HWND)-1
	SWP_NOSIZE        = 0x0001
	SWP_NOMOVE        = 0x0002
	SWP_NOACTIVATE    = 0x0010
	SWP_SHOWWINDOW    = 0x0040
	PS_SOLID          = 0
	RGN_COPY          = 5
	ALTERNATE         = 1
	GWLP_WNDPROC      = ^uintptr(3) // -4
	ASFW_ANY          = ^uintptr(0)
	OFN_FILEMUSTEXIST = 0x00001000
	DWMWA_DARK_MODE   = 20
	DWMWA_CORNER_PREF = 33
	DWMWCP_ROUND      = 2
	DWMWCP_ROUNDSMALL = 3
	WS_EX_LAYERED     = 0x00080000
	LWA_ALPHA         = 0x2
)

// CreatePen creates a solid GDI pen.
func CreatePen(width int32, color uint32) uintptr {
	r, _, _ := pCreatePen.Call(PS_SOLID, uintptr(width), uintptr(color))
	return r
}

// Polyline draws connected segments with the selected pen.
func Polyline(dc uintptr, pts []Point) {
	if len(pts) < 2 {
		return
	}
	pPolyline.Call(dc, uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
}

// GdiFlush finishes pending GDI drawing, before a DIB's bits are touched directly.
func GdiFlush() { pGdiFlush.Call() }

// ClipPolygon restricts drawing on dc to a polygon; pass nil to clear.
func ClipPolygon(dc uintptr, pts []Point) {
	if len(pts) < 3 {
		pSelectClipRgn.Call(dc, 0)
		return
	}
	rgn, _, _ := pCreatePolygonRgn.Call(uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)), ALTERNATE)
	pSelectClipRgn.Call(dc, rgn)
	pDeleteObject.Call(rgn)
}

// SubclassWindow replaces a window procedure and returns the previous one.
func SubclassWindow(h windows.HWND, proc uintptr) uintptr {
	r, _, _ := pSetWindowLongPtrW.Call(uintptr(h), GWLP_WNDPROC, proc)
	return r
}

// CallWindowProc forwards a message to a previous window procedure.
func CallWindowProc(prev uintptr, h windows.HWND, msg uint32, w, l uintptr) uintptr {
	r, _, _ := pCallWindowProcW.Call(prev, uintptr(h), uintptr(msg), w, l)
	return r
}

// AllowAnyForeground lets the next process we signal take the foreground.
func AllowAnyForeground() { pAllowSetForegroundWnd.Call(ASFW_ANY) }

// SetTopmost shows a window topmost at a position and size.
func SetTopmost(h windows.HWND, x, y, w, hgt int32, activate bool) {
	flags := uintptr(SWP_SHOWWINDOW)
	if !activate {
		flags |= SWP_NOACTIVATE
	}
	pSetWindowPos.Call(uintptr(h), HWND_TOPMOST, uintptr(x), uintptr(y), uintptr(w), uintptr(hgt), flags)
}

// BringToFront restores and activates a window.
func BringToFront(h windows.HWND) {
	ShowWindow(h, SW_RESTORE)
	pBringWindowToTop.Call(uintptr(h))
	SetForegroundWindow(h)
}

// IsVisible reports whether a window is visible.
func IsVisible(h windows.HWND) bool {
	r, _, _ := pIsWindowVisibleExport.Call(uintptr(h))
	return r != 0
}

// SetDarkTitleBar switches the caption to dark or light.
func SetDarkTitleBar(h windows.HWND, dark bool) {
	v := int32(0)
	if dark {
		v = 1
	}
	pDwmSetWindowAttribute.Call(uintptr(h), DWMWA_DARK_MODE, uintptr(unsafe.Pointer(&v)), 4)
}

// RoundCorners asks DWM for Windows 11 rounded corners (and the system shadow).
func RoundCorners(h windows.HWND, small bool) {
	v := int32(DWMWCP_ROUND)
	if small {
		v = DWMWCP_ROUNDSMALL
	}
	pDwmSetWindowAttribute.Call(uintptr(h), DWMWA_CORNER_PREF, uintptr(unsafe.Pointer(&v)), 4)
}

// SetWindowAlpha sets whole-window opacity of a WS_EX_LAYERED window.
func SetWindowAlpha(h windows.HWND, a uint8) {
	pSetLayeredWindowAttrib.Call(uintptr(h), 0, uintptr(a), LWA_ALPHA)
}

// OpenFileDialog shows the standard Open dialog and returns the chosen path or "".
func OpenFileDialog(owner windows.HWND, title, filter string) string {
	buf := make([]uint16, 32768)
	f := MultiString(filter)
	ofn := OpenFileName{
		Owner: owner, Filter: &f[0], FilterIndex: 1, File: &buf[0], MaxFile: uint32(len(buf)),
		Title: Str(title), Flags: OFN_FILEMUSTEXIST | OFN_PATHMUSTEXIST | OFN_EXPLORER | OFN_NOCHANGEDIR,
	}
	ofn.StructSize = uint32(unsafe.Sizeof(ofn))
	if r, _, _ := pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn))); r == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

// Window messages and constants.
const (
	WM_DESTROY        = 0x0002
	WM_SIZE           = 0x0005
	WM_SETFOCUS       = 0x0007
	WM_PAINT          = 0x000F
	WM_CLOSE          = 0x0010
	WM_ERASEBKGND     = 0x0014
	WM_SETCURSOR      = 0x0020
	WM_COPYDATA       = 0x004A
	WM_KEYDOWN        = 0x0100
	WM_KEYUP          = 0x0101
	WM_SYSKEYDOWN     = 0x0104
	WM_COMMAND        = 0x0111
	WM_TIMER          = 0x0113
	WM_MOUSEMOVE      = 0x0200
	WM_LBUTTONDOWN    = 0x0201
	WM_LBUTTONUP      = 0x0202
	WM_RBUTTONDOWN    = 0x0204
	WM_RBUTTONUP      = 0x0205
	WM_CONTEXTMENU    = 0x007B
	WM_HOTKEY         = 0x0312
	WM_APP            = 0x8000
	WM_USER           = 0x0400
	WM_NULL           = 0x0000
	WM_CAPTURECHANGED = 0x0215

	WS_POPUP          = 0x80000000
	WS_OVERLAPPED     = 0x00000000
	WS_EX_TOPMOST     = 0x00000008
	WS_EX_TOOLWINDOW  = 0x00000080
	WS_EX_NOACTIVATE  = 0x08000000
	SW_HIDE           = 0
	SW_SHOW           = 5
	SW_SHOWNORMAL     = 1
	CS_HREDRAW        = 0x0002
	CS_VREDRAW        = 0x0001
	IDC_ARROW         = 32512
	IDC_CROSS         = 32515
	IDC_HAND          = 32649
	IDC_SIZENWSE      = 32642
	IDC_SIZENESW      = 32643
	IDC_SIZEWE        = 32644
	IDC_SIZENS        = 32645
	SRCCOPY           = 0x00CC0020
	CAPTUREBLT        = 0x40000000
	DIB_RGB_COLORS    = 0
	BI_RGB            = 0
	TRANSPARENT       = 1
	DT_CENTER         = 0x0001
	DT_VCENTER        = 0x0004
	DT_SINGLELINE     = 0x0020
	DT_NOPREFIX       = 0x0800
	DT_LEFT           = 0x0000
	DI_NORMAL         = 0x0003
	GWL_EXSTYLE       = -20
	GW_OWNER          = 4
	MF_STRING         = 0x0000
	MF_SEPARATOR      = 0x0800
	MF_CHECKED        = 0x0008
	MF_GRAYED         = 0x0001
	TPM_RIGHTBUTTON   = 0x0002
	TPM_RETURNCMD     = 0x0100
	TPM_BOTTOMALIGN   = 0x0020
	MB_OK             = 0x0000
	MB_ICONERROR      = 0x0010
	MB_ICONINFO       = 0x0040
	MB_ICONWARNING    = 0x0030
	MB_YESNO          = 0x0004
	IDYES             = 6
	VK_SHIFT          = 0x10
	VK_CONTROL        = 0x11
	VK_ESCAPE         = 0x1B
	VK_RETURN         = 0x0D
	VK_LEFT           = 0x25
	VK_UP             = 0x26
	VK_RIGHT          = 0x27
	VK_DOWN           = 0x28
	SM_XVIRTUALSCREEN = 76
	SM_YVIRTUALSCREEN = 77
	SM_CXVIRTUALSCREN = 78
	SM_CYVIRTUALSCREN = 79
	MONITORINFOF_PRIM = 1
	CF_UNICODETEXT    = 13
	CF_DIB            = 8
	GMEM_MOVEABLE     = 0x0002
	CURSOR_SHOWING    = 0x00000001

	DWMWA_EXTENDED_FRAME_BOUNDS = 9
	DWMWA_CLOAKED               = 14

	NIM_ADD              = 0
	NIM_MODIFY           = 1
	NIM_DELETE           = 2
	NIM_SETVERSION       = 4
	NIF_MESSAGE          = 0x01
	NIF_ICON             = 0x02
	NIF_TIP              = 0x04
	NIF_INFO             = 0x10
	NIF_SHOWTIP          = 0x80
	NIIF_INFO            = 0x01
	NIIF_WARNING         = 0x02
	NIN_SELECT           = WM_USER + 0
	NIN_BALLOONUSERCLICK = WM_USER + 5
	NOTIFYICON_VERSION4  = 4

	OFN_OVERWRITEPROMPT = 0x00000002
	OFN_PATHMUSTEXIST   = 0x00000800
	OFN_EXPLORER        = 0x00080000
	OFN_NOCHANGEDIR     = 0x00000008

	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2
	DpiPerMonitorV2 = ^uintptr(3) // (DPI_AWARENESS_CONTEXT)-4
)

// Point is a Win32 POINT.
type Point struct{ X, Y int32 }

// Rect is a Win32 RECT.
type Rect struct{ Left, Top, Right, Bottom int32 }

// Msg is a Win32 MSG.
type Msg struct {
	Hwnd    windows.HWND
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      Point
	Private uint32
}

// WndClassEx is WNDCLASSEXW.
type WndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

// PaintStruct is PAINTSTRUCT.
type PaintStruct struct {
	Hdc         uintptr
	Erase       int32
	Paint       Rect
	Restore     int32
	IncUpdate   int32
	RgbReserved [32]byte
}

// BitmapInfoHeader is BITMAPINFOHEADER.
type BitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// BitmapInfo is BITMAPINFO with one (unused) color entry.
type BitmapInfo struct {
	Header BitmapInfoHeader
	Colors [1]uint32
}

// MonitorInfoEx is MONITORINFOEXW.
type MonitorInfoEx struct {
	Size    uint32
	Monitor Rect
	Work    Rect
	Flags   uint32
	Device  [32]uint16
}

// NotifyIconData is NOTIFYICONDATAW (Vista+ layout).
type NotifyIconData struct {
	Size            uint32
	Wnd             windows.HWND
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            windows.Handle
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GuidItem        windows.GUID
	BalloonIcon     windows.Handle
}

// CursorInfo is CURSORINFO.
type CursorInfo struct {
	Size   uint32
	Flags  uint32
	Cursor windows.Handle
	Pt     Point
}

// IconInfo is ICONINFO.
type IconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	Mask     windows.Handle
	Color    windows.Handle
}

// CopyDataStruct is COPYDATASTRUCT.
type CopyDataStruct struct {
	Data uintptr
	Size uint32
	Ptr  uintptr
}

// OpenFileName is OPENFILENAMEW.
type OpenFileName struct {
	StructSize    uint32
	Owner         windows.HWND
	Instance      windows.Handle
	Filter        *uint16
	CustomFilter  *uint16
	MaxCustFilter uint32
	FilterIndex   uint32
	File          *uint16
	MaxFile       uint32
	FileTitle     *uint16
	MaxFileTitle  uint32
	InitialDir    *uint16
	Title         *uint16
	Flags         uint32
	FileOffset    uint16
	FileExtension uint16
	DefExt        *uint16
	CustData      uintptr
	Hook          uintptr
	TemplateName  *uint16
	Reserved      uintptr
	Reserved2     uint32
	FlagsEx       uint32
}

// Str converts a Go string to a NUL-terminated UTF-16 pointer.
func Str(s string) *uint16 {
	p, _ := windows.UTF16PtrFromString(s)
	return p
}

func ptr(s string) uintptr { return uintptr(unsafe.Pointer(Str(s))) }

// RGB packs a COLORREF.
func RGB(r, g, b uint8) uint32 { return uint32(r) | uint32(g)<<8 | uint32(b)<<16 }

// LoWord / HiWord helpers for lParam coordinates (signed, multi-monitor safe).
func LoInt(l uintptr) int32 { return int32(int16(uint16(l))) }
func HiInt(l uintptr) int32 { return int32(int16(uint16(l >> 16))) }

// ---- process / module ----

func SetDpiAwarenessPerMonitorV2() bool {
	if pSetProcessDpiAwarenessCtx.Find() != nil {
		return false
	}
	r, _, _ := pSetProcessDpiAwarenessCtx.Call(DpiPerMonitorV2)
	return r != 0
}

func ModuleHandle() windows.Handle {
	r, _, _ := pGetModuleHandleW.Call(0)
	return windows.Handle(r)
}

// ---- windows ----

func RegisterClass(name string, proc uintptr, cursor uintptr, icon windows.Handle) error {
	return RegisterClassStyle(name, proc, cursor, icon, CS_HREDRAW|CS_VREDRAW)
}

// CS_DROPSHADOW gives popup windows the system drop shadow.
const CS_DROPSHADOW = 0x00020000

// RegisterClassStyle registers a window class with an explicit class style.
func RegisterClassStyle(name string, proc uintptr, cursor uintptr, icon windows.Handle, style uint32) error {
	wc := WndClassEx{
		Style:     style,
		WndProc:   proc,
		Instance:  ModuleHandle(),
		Cursor:    windows.Handle(LoadCursor(cursor)),
		ClassName: Str(name),
		Icon:      icon,
		IconSm:    icon,
	}
	wc.Size = uint32(unsafe.Sizeof(wc))
	r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if r == 0 && err != windows.ERROR_CLASS_ALREADY_EXISTS {
		return err
	}
	return nil
}

func CreateWindow(exStyle uint32, class, title string, style uint32, x, y, w, h int32) (windows.HWND, error) {
	r, _, err := pCreateWindowExW.Call(uintptr(exStyle), ptr(class), ptr(title), uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0, 0, uintptr(ModuleHandle()), 0)
	if r == 0 {
		return 0, err
	}
	return windows.HWND(r), nil
}

// CreateMessageWindow creates an invisible message-only window (parent HWND_MESSAGE).
func CreateMessageWindow() (windows.HWND, error) {
	const hwndMessage = ^uintptr(2) // (HWND)-3
	r, _, err := pCreateWindowExW.Call(0, ptr("STATIC"), ptr("SnippingToolEnhanced.Clipboard"), 0,
		0, 0, 0, 0, hwndMessage, 0, uintptr(ModuleHandle()), 0)
	if r == 0 {
		return 0, err
	}
	return windows.HWND(r), nil
}

func DefWindowProc(h windows.HWND, msg uint32, w, l uintptr) uintptr {
	r, _, _ := pDefWindowProcW.Call(uintptr(h), uintptr(msg), w, l)
	return r
}

func DestroyWindow(h windows.HWND)     { pDestroyWindow.Call(uintptr(h)) }
func ShowWindow(h windows.HWND, c int) { pShowWindow.Call(uintptr(h), uintptr(c)) }
func UpdateWindow(h windows.HWND)      { pUpdateWindow.Call(uintptr(h)) }
func PostQuitMessage(code int)         { pPostQuitMessage.Call(uintptr(code)) }
func SetForegroundWindow(h windows.HWND) bool {
	r, _, _ := pSetForegroundWindow.Call(uintptr(h))
	return r != 0
}
func SetFocus(h windows.HWND) { pSetFocus.Call(uintptr(h)) }
func PostMessage(h windows.HWND, msg uint32, w, l uintptr) {
	pPostMessageW.Call(uintptr(h), uintptr(msg), w, l)
}
func SendMessage(h windows.HWND, msg uint32, w, l uintptr) uintptr {
	r, _, _ := pSendMessageW.Call(uintptr(h), uintptr(msg), w, l)
	return r
}

// GetMessage returns false on WM_QUIT or error.
func GetMessage(m *Msg) bool {
	r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(m)), 0, 0, 0)
	return int32(r) > 0
}
func TranslateMessage(m *Msg) { pTranslateMessage.Call(uintptr(unsafe.Pointer(m))) }
func DispatchMessage(m *Msg)  { pDispatchMessageW.Call(uintptr(unsafe.Pointer(m))) }

func Invalidate(h windows.HWND) { pInvalidateRect.Call(uintptr(h), 0, 0) }

// InvalidateRect marks part of a window for repaint.
func InvalidateRect(h windows.HWND, r Rect) {
	pInvalidateRect.Call(uintptr(h), uintptr(unsafe.Pointer(&r)), 0)
}
func BeginPaint(h windows.HWND, ps *PaintStruct) uintptr {
	r, _, _ := pBeginPaint.Call(uintptr(h), uintptr(unsafe.Pointer(ps)))
	return r
}
func EndPaint(h windows.HWND, ps *PaintStruct) {
	pEndPaint.Call(uintptr(h), uintptr(unsafe.Pointer(ps)))
}
func SetCapture(h windows.HWND) { pSetCapture.Call(uintptr(h)) }
func ReleaseCapture()           { pReleaseCapture.Call() }
func LoadCursor(id uintptr) uintptr {
	r, _, _ := pLoadCursorW.Call(0, id)
	return r
}
func SetCursor(c uintptr) { pSetCursor.Call(c) }
func GetSystemMetrics(i int) int32 {
	r, _, _ := pGetSystemMetrics.Call(uintptr(i))
	return int32(r)
}
func KeyDown(vk int) bool {
	r, _, _ := pGetKeyState.Call(uintptr(vk))
	return int16(r) < 0
}
func DpiForWindow(h windows.HWND) uint32 {
	if pGetDpiForWindow.Find() != nil {
		return 96
	}
	r, _, _ := pGetDpiForWindow.Call(uintptr(h))
	if r == 0 {
		return 96
	}
	return uint32(r)
}

// SystemDPI returns the system DPI (96 = 100%).
func SystemDPI() uint32 {
	if pGetDpiForSystem.Find() != nil {
		return 96
	}
	r, _, _ := pGetDpiForSystem.Call()
	if r == 0 {
		return 96
	}
	return uint32(r)
}

func CursorPos() Point {
	var p Point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p
}
func SetTimer(h windows.HWND, id, ms uintptr) { pSetTimer.Call(uintptr(h), id, ms, 0) }
func KillTimer(h windows.HWND, id uintptr)    { pKillTimer.Call(uintptr(h), id) }
func FindWindow(class string) windows.HWND {
	r, _, _ := pFindWindowW.Call(ptr(class), 0)
	return windows.HWND(r)
}
func MessageBox(h windows.HWND, text, caption string, flags uint32) int {
	r, _, _ := pMessageBoxW.Call(uintptr(h), ptr(text), ptr(caption), uintptr(flags))
	return int(r)
}
func RegisterWindowMessage(name string) uint32 {
	r, _, _ := pRegisterWindowMessageW.Call(ptr(name))
	return uint32(r)
}
func RegisterHotKey(h windows.HWND, id int, mods, vk uint32) error {
	r, _, err := pRegisterHotKey.Call(uintptr(h), uintptr(id), uintptr(mods), uintptr(vk))
	if r == 0 {
		return err
	}
	return nil
}
func UnregisterHotKey(h windows.HWND, id int) { pUnregisterHotKey.Call(uintptr(h), uintptr(id)) }

// ---- menus ----

func CreatePopupMenu() uintptr { r, _, _ := pCreatePopupMenu.Call(); return r }
func AppendMenu(m uintptr, flags uint32, id uintptr, text string) {
	if flags&MF_SEPARATOR != 0 {
		pAppendMenuW.Call(m, uintptr(flags), 0, 0)
		return
	}
	pAppendMenuW.Call(m, uintptr(flags), id, ptr(text))
}
func TrackPopupMenu(m uintptr, h windows.HWND, x, y int32) uintptr {
	r, _, _ := pTrackPopupMenu.Call(m, TPM_RIGHTBUTTON|TPM_RETURNCMD|TPM_BOTTOMALIGN, uintptr(x), uintptr(y), 0, uintptr(h), 0)
	return r
}
func DestroyMenu(m uintptr) { pDestroyMenu.Call(m) }

// ---- monitors and windows ----

// Monitor describes one display in virtual-screen pixel coordinates.
type Monitor struct {
	Rect    Rect
	Work    Rect
	Primary bool
	Name    string
}

var monitorAcc []Monitor
var monitorCB = syscall.NewCallback(func(hmon, hdc, lprc, data uintptr) uintptr {
	var mi MonitorInfoEx
	mi.Size = uint32(unsafe.Sizeof(mi))
	if r, _, _ := pGetMonitorInfoW.Call(hmon, uintptr(unsafe.Pointer(&mi))); r != 0 {
		monitorAcc = append(monitorAcc, Monitor{
			Rect: mi.Monitor, Work: mi.Work, Primary: mi.Flags&MONITORINFOF_PRIM != 0,
			Name: windows.UTF16ToString(mi.Device[:]),
		})
	}
	return 1
})

// Monitors lists all displays. Not safe for concurrent use (UI thread only).
func Monitors() []Monitor {
	monitorAcc = nil
	pEnumDisplayMonitors.Call(0, 0, monitorCB, 0)
	return monitorAcc
}

// TopWindow describes a capturable top-level window.
type TopWindow struct {
	Hwnd  windows.HWND
	Rect  Rect
	Title string
	Class string
	PID   uint32
}

var windowAcc []TopWindow
var windowsCB = syscall.NewCallback(func(h, data uintptr) uintptr {
	hwnd := windows.HWND(h)
	if v, _, _ := pIsWindowVisible.Call(h); v == 0 {
		return 1
	}
	if v, _, _ := pIsIconic.Call(h); v != 0 {
		return 1
	}
	var cloaked uint32
	pDwmGetWindowAttribute.Call(h, DWMWA_CLOAKED, uintptr(unsafe.Pointer(&cloaked)), 4)
	if cloaked != 0 {
		return 1
	}
	var r Rect
	if hr, _, _ := pDwmGetWindowAttribute.Call(h, DWMWA_EXTENDED_FRAME_BOUNDS, uintptr(unsafe.Pointer(&r)), unsafe.Sizeof(r)); hr != 0 {
		return 1
	}
	if r.Right-r.Left < 8 || r.Bottom-r.Top < 8 {
		return 1
	}
	var title [256]uint16
	pGetWindowTextW.Call(h, uintptr(unsafe.Pointer(&title[0])), 256)
	var class [256]uint16
	pGetClassNameW.Call(h, uintptr(unsafe.Pointer(&class[0])), 256)
	ex, _, _ := pGetWindowLongPtrW.Call(h, uintptr(^uintptr(19))) // GWL_EXSTYLE (-20)
	t := windows.UTF16ToString(title[:])
	if ex&WS_EX_TOOLWINDOW != 0 && t == "" {
		return 1
	}
	var pid uint32
	pGetWindowThreadProcessId.Call(h, uintptr(unsafe.Pointer(&pid)))
	windowAcc = append(windowAcc, TopWindow{Hwnd: hwnd, Rect: r, Title: t, Class: windows.UTF16ToString(class[:]), PID: pid})
	return 1
})

// TopLevelWindows returns visible windows in z-order (topmost first).
func TopLevelWindows() []TopWindow {
	windowAcc = nil
	pEnumWindows.Call(windowsCB, 0)
	return windowAcc
}

// ---- GDI ----

// DIB is a top-down 32-bpp DIB section selected into its own memory DC.
type DIB struct {
	DC     uintptr
	Bitmap uintptr
	old    uintptr
	Bits   []byte
	W, H   int
}

// NewDIB creates a DIB section of w×h pixels.
func NewDIB(w, h int) (*DIB, error) {
	screen, _, _ := pGetDC.Call(0)
	defer pReleaseDC.Call(0, screen)
	dc, _, _ := pCreateCompatibleDC.Call(screen)
	if dc == 0 {
		return nil, windows.GetLastError()
	}
	bi := BitmapInfo{Header: BitmapInfoHeader{Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32, Compression: BI_RGB}}
	bi.Header.Size = uint32(unsafe.Sizeof(bi.Header))
	var bits unsafe.Pointer
	bmp, _, err := pCreateDIBSection.Call(dc, uintptr(unsafe.Pointer(&bi)), DIB_RGB_COLORS, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 {
		pDeleteDC.Call(dc)
		return nil, err
	}
	old, _, _ := pSelectObject.Call(dc, bmp)
	return &DIB{DC: dc, Bitmap: bmp, old: old, Bits: unsafe.Slice((*byte)(bits), w*h*4), W: w, H: h}, nil
}

// Close frees the DIB.
func (d *DIB) Close() {
	if d == nil || d.DC == 0 {
		return
	}
	pSelectObject.Call(d.DC, d.old)
	pDeleteObject.Call(d.Bitmap)
	pDeleteDC.Call(d.DC)
	d.DC, d.Bits = 0, nil
}

// BlitFromScreen copies a screen rectangle into the DIB at (0,0).
func (d *DIB) BlitFromScreen(x, y int32) error {
	screen, _, _ := pGetDC.Call(0)
	defer pReleaseDC.Call(0, screen)
	r, _, err := pBitBlt.Call(d.DC, 0, 0, uintptr(d.W), uintptr(d.H), screen, uintptr(x), uintptr(y), SRCCOPY|CAPTUREBLT)
	if r == 0 {
		return err
	}
	return nil
}

func BitBlt(dst uintptr, x, y, w, h int32, src uintptr, sx, sy int32) {
	pBitBlt.Call(dst, uintptr(x), uintptr(y), uintptr(w), uintptr(h), src, uintptr(sx), uintptr(sy), SRCCOPY)
}

func FillRect(dc uintptr, r Rect, color uint32) {
	br, _, _ := pCreateSolidBrush.Call(uintptr(color))
	pFillRect.Call(dc, uintptr(unsafe.Pointer(&r)), br)
	pDeleteObject.Call(br)
}

// FrameRect draws a rectangle outline of thickness t.
func FrameRect(dc uintptr, r Rect, color uint32, t int32) {
	FillRect(dc, Rect{r.Left, r.Top, r.Right, r.Top + t}, color)
	FillRect(dc, Rect{r.Left, r.Bottom - t, r.Right, r.Bottom}, color)
	FillRect(dc, Rect{r.Left, r.Top, r.Left + t, r.Bottom}, color)
	FillRect(dc, Rect{r.Right - t, r.Top, r.Right, r.Bottom}, color)
}

// Font creates a Segoe UI font of the given pixel height.
func Font(px int32, bold bool) uintptr {
	weight := 400
	if bold {
		weight = 600
	}
	r, _, _ := pCreateFontW.Call(uintptr(-px), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5 /*CLEARTYPE*/, 0, ptr("Segoe UI"))
	return r
}

func SelectObject(dc, obj uintptr) uintptr { r, _, _ := pSelectObject.Call(dc, obj); return r }
func DeleteObject(obj uintptr)             { pDeleteObject.Call(obj) }

// DrawText draws text in r.
func DrawText(dc uintptr, s string, r Rect, color uint32, flags uint32) {
	pSetBkMode.Call(dc, TRANSPARENT)
	pSetTextColor.Call(dc, uintptr(color))
	u, err := windows.UTF16FromString(s)
	if err != nil || len(u) < 2 {
		return
	}
	pDrawTextW.Call(dc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&r)), uintptr(flags|DT_NOPREFIX))
}

func GetDC(h windows.HWND) uintptr         { r, _, _ := pGetDC.Call(uintptr(h)); return r }
func ReleaseDC(h windows.HWND, dc uintptr) { pReleaseDC.Call(uintptr(h), dc) }

// DrawCursor paints the current mouse cursor into dc, offset by (ox,oy).
func DrawCursor(dc uintptr, ox, oy int32) {
	var ci CursorInfo
	ci.Size = uint32(unsafe.Sizeof(ci))
	if r, _, _ := pGetCursorInfo.Call(uintptr(unsafe.Pointer(&ci))); r == 0 || ci.Flags&CURSOR_SHOWING == 0 {
		return
	}
	var ii IconInfo
	if r, _, _ := pGetIconInfo.Call(uintptr(ci.Cursor), uintptr(unsafe.Pointer(&ii))); r != 0 {
		if ii.Mask != 0 {
			pDeleteObject.Call(uintptr(ii.Mask))
		}
		if ii.Color != 0 {
			pDeleteObject.Call(uintptr(ii.Color))
		}
	}
	pDrawIconEx.Call(dc, uintptr(ci.Pt.X-ox-int32(ii.XHotspot)), uintptr(ci.Pt.Y-oy-int32(ii.YHotspot)), uintptr(ci.Cursor), 0, 0, 0, 0, DI_NORMAL)
}

// IconFromBGRA builds an HICON from a top-down BGRA (premultiplied not
// required) square image.
func IconFromBGRA(bgra []byte, size int) windows.Handle {
	d, err := NewDIB(size, size)
	if err != nil {
		return 0
	}
	copy(d.Bits, bgra)
	pSelectObject.Call(d.DC, d.old) // deselect before use in icon
	// All-zero AND mask: the color bitmap's alpha channel decides transparency.
	maskBits := make([]byte, ((size+15)/16*2)*size)
	mask, _, _ := pCreateBitmap.Call(uintptr(size), uintptr(size), 1, 1, uintptr(unsafe.Pointer(&maskBits[0])))
	ii := IconInfo{FIcon: 1, Mask: windows.Handle(mask), Color: windows.Handle(d.Bitmap)}
	h, _, _ := pCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))
	pDeleteObject.Call(mask)
	pDeleteObject.Call(d.Bitmap)
	pDeleteDC.Call(d.DC)
	return windows.Handle(h)
}

// ---- clipboard primitives ----

func OpenClipboard(h windows.HWND) bool {
	r, _, _ := pOpenClipboard.Call(uintptr(h))
	return r != 0
}
func CloseClipboard() { pCloseClipboard.Call() }
func EmptyClipboard() { pEmptyClipboard.Call() }
func RegisterClipboardFormat(name string) uint32 {
	r, _, _ := pRegisterClipboardFormatW.Call(ptr(name))
	return uint32(r)
}

// SetClipboardBytes copies data into a movable global and hands it to the clipboard.
func SetClipboardBytes(format uint32, data []byte) error {
	h, _, err := pGlobalAlloc.Call(GMEM_MOVEABLE, uintptr(len(data)))
	if h == 0 {
		return err
	}
	p, _, err := pGlobalLock.Call(h)
	if p == 0 {
		pGlobalFree.Call(h)
		return err
	}
	copy(unsafe.Slice((*byte)(PtrFromUintptr(p)), len(data)), data)
	pGlobalUnlock.Call(h)
	if r, _, err := pSetClipboardData.Call(uintptr(format), h); r == 0 {
		pGlobalFree.Call(h)
		return err
	}
	return nil
}

// ---- shell ----

func NotifyIcon(op uint32, nid *NotifyIconData) bool {
	nid.Size = uint32(unsafe.Sizeof(*nid))
	r, _, _ := pShellNotifyIconW.Call(uintptr(op), uintptr(unsafe.Pointer(nid)))
	return r != 0
}

// ShellOpen opens a file, folder or URI with its default handler.
func ShellOpen(target string) error {
	r, _, _ := pShellExecuteW.Call(0, ptr("open"), ptr(target), 0, 0, SW_SHOWNORMAL)
	if r <= 32 {
		return syscall.Errno(r)
	}
	return nil
}

// CopyString fills a fixed UTF-16 buffer (truncating).
func CopyString(dst []uint16, s string) {
	u, _ := windows.UTF16FromString(s)
	if len(u) > len(dst) {
		u = u[:len(dst)]
		u[len(u)-1] = 0
	}
	copy(dst, u)
}

// SaveFileDialog shows the standard Save As dialog. filter uses "Name\x00*.ext\x00" pairs.
// Returns the chosen path and the 1-based filter index, or "" if cancelled.
func SaveFileDialog(owner windows.HWND, title, initialDir, defaultName, filter, defExt string) (string, int) {
	buf := make([]uint16, 32768)
	CopyString(buf, defaultName)
	f := MultiString(filter)
	ofn := OpenFileName{
		Owner: owner, Filter: &f[0], FilterIndex: 1, File: &buf[0], MaxFile: uint32(len(buf)),
		InitialDir: Str(initialDir), Title: Str(title), DefExt: Str(defExt),
		Flags: OFN_OVERWRITEPROMPT | OFN_PATHMUSTEXIST | OFN_EXPLORER | OFN_NOCHANGEDIR,
	}
	ofn.StructSize = uint32(unsafe.Sizeof(ofn))
	r, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return "", 0
	}
	return windows.UTF16ToString(buf), int(ofn.FilterIndex)
}

// PtrFromUintptr converts a pointer returned by a Win32 call (as uintptr)
// back into an unsafe.Pointer. The memory is owned by Windows, not the Go heap.
func PtrFromUintptr(p uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&p))
}

// AppsUseDarkTheme reads the Windows app theme (Settings > Personalization > Colors).
func AppsUseDarkTheme() bool {
	var k windows.Handle
	path, _ := windows.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
	if windows.RegOpenKeyEx(windows.HKEY_CURRENT_USER, path, 0, windows.KEY_QUERY_VALUE, &k) != nil {
		return false
	}
	defer windows.RegCloseKey(k)
	var v, typ uint32
	n := uint32(4)
	name, _ := windows.UTF16PtrFromString("AppsUseLightTheme")
	if windows.RegQueryValueEx(k, name, nil, &typ, (*byte)(unsafe.Pointer(&v)), &n) != nil {
		return false
	}
	return v == 0
}

const (
	WM_MOUSEWHEEL = 0x020A
	VK_TAB        = 0x09
	VK_OEM_PLUS   = 0xBB
	VK_OEM_MINUS  = 0xBD
	VK_ADD        = 0x6B
	VK_SUBTRACT   = 0x6D
)

// MoveWindow moves and resizes a window.
func MoveWindow(h windows.HWND, x, y, w, hgt int32) {
	pMoveWindow.Call(uintptr(h), uintptr(x), uintptr(y), uintptr(w), uintptr(hgt), 1)
}

// PrimaryWorkArea returns the work area of the primary monitor.
func PrimaryWorkArea() Rect {
	for _, m := range Monitors() {
		if m.Primary {
			return m.Work
		}
	}
	return Rect{Right: 1920, Bottom: 1040}
}
