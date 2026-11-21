// SPDX-License-Identifier: Apache-2.0

//go:build windows

package ocr

import (
	"errors"
	"fmt"
	"image"
	"image/draw"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"snippingtoolenhanced/internal/win"
)

// Interface IDs and vtable indices come from the Windows SDK metadata
// (Windows.winmd, as published in microsoft/windows-rs).
var (
	combase                       = windows.NewLazySystemDLL("combase.dll")
	pRoInitialize                 = combase.NewProc("RoInitialize")
	pRoGetActivationFactory       = combase.NewProc("RoGetActivationFactory")
	pWindowsCreateString          = combase.NewProc("WindowsCreateString")
	pWindowsDeleteString          = combase.NewProc("WindowsDeleteString")
	pWindowsGetStringRawBuffer    = combase.NewProc("WindowsGetStringRawBuffer")
	iidOcrEngineStatics           = win.MustGUID("5bffa85a-3384-3540-9940-699120d428a8")
	iidSoftwareBitmapStatics      = win.MustGUID("df0385db-672f-4a9d-806e-c2442f343e86")
	iidCryptographicBufferStatics = win.MustGUID("320b7e22-3cb0-4cdf-8663-1d28910065eb")
	iidAsyncInfo                  = win.MustGUID("00000036-0000-0000-c000-000000000046")
)

const (
	// IInspectable has 6 methods; interface methods start at 6.
	ocrStaticsMaxImageDimension = 6
	ocrStaticsTryCreateFromUser = 10
	ocrEngineRecognizeAsync     = 6
	ocrResultLines              = 6
	ocrResultTextAngle          = 7
	ocrLineWords                = 6
	ocrLineText                 = 7
	ocrWordBoundingRect         = 6
	ocrWordText                 = 7
	sbStaticsCreateCopyFromBuf  = 9
	cbStaticsCreateFromByteArr  = 9
	asyncInfoStatus             = 7
	asyncInfoErrorCode          = 8
	asyncOpGetResults           = 8
	vectorGetAt                 = 6
	vectorSize                  = 7
	ireferenceValue             = 6

	pixelFormatBgra8 = 87
	asyncStarted     = 0
	asyncCompleted   = 1
	roInitMulti      = 1
)

// ErrNoLanguage means no OCR language is installed for the user's languages.
var ErrNoLanguage = errors.New("no text recognition (OCR) language is installed. Add one in Settings > Time & language > Language & region")

type job struct {
	img  image.Image
	done chan jobResult
}

type jobResult struct {
	res Result
	err error
}

var (
	once sync.Once
	jobs = make(chan job)
)

// Recognize finds the text in img. Calls are serialized on one worker thread
// that keeps the OCR engine loaded.
func Recognize(img image.Image) (Result, error) {
	once.Do(func() { go worker() })
	j := job{img: img, done: make(chan jobResult, 1)}
	jobs <- j
	r := <-j.done
	return r.res, r.err
}

func worker() {
	runtime.LockOSThread()
	pRoInitialize.Call(roInitMulti)
	var engine *win.COM
	var maxDim uint32
	var initErr error
	for j := range jobs {
		if engine == nil && initErr == nil {
			engine, maxDim, initErr = createEngine()
		}
		if initErr != nil {
			j.done <- jobResult{err: initErr}
			continue
		}
		res, err := recognize(engine, maxDim, j.img)
		j.done <- jobResult{res: res, err: err}
	}
}

func hstring(s string) (uintptr, error) {
	u, err := windows.UTF16FromString(s)
	if err != nil {
		return 0, err
	}
	var h uintptr
	r, _, _ := pWindowsCreateString.Call(uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&h)))
	return h, win.HRErr("WindowsCreateString", r)
}

func goString(h uintptr) string {
	if h == 0 {
		return ""
	}
	var n uint32
	p, _, _ := pWindowsGetStringRawBuffer.Call(h, uintptr(unsafe.Pointer(&n)))
	if p == 0 || n == 0 {
		return ""
	}
	return windows.UTF16ToString(unsafe.Slice((*uint16)(win.PtrFromUintptr(p)), n))
}

func factory(class string, iid *windows.GUID) (*win.COM, error) {
	h, err := hstring(class)
	if err != nil {
		return nil, err
	}
	defer pWindowsDeleteString.Call(h)
	var f *win.COM
	r, _, _ := pRoGetActivationFactory.Call(h, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&f)))
	if err := win.HRErr("RoGetActivationFactory("+class+")", r); err != nil {
		return nil, err
	}
	return f, nil
}

func createEngine() (*win.COM, uint32, error) {
	st, err := factory("Windows.Media.Ocr.OcrEngine", &iidOcrEngineStatics)
	if err != nil {
		return nil, 0, err
	}
	defer st.Release()
	var maxDim uint32
	st.Call(ocrStaticsMaxImageDimension, uintptr(unsafe.Pointer(&maxDim)))
	if maxDim == 0 {
		maxDim = 4096
	}
	var e *win.COM
	if err := st.HR("TryCreateFromUserProfileLanguages", ocrStaticsTryCreateFromUser, uintptr(unsafe.Pointer(&e))); err != nil {
		return nil, 0, err
	}
	if e == nil {
		return nil, 0, ErrNoLanguage
	}
	return e, maxDim, nil
}

// prepare converts to BGRA and scales: small text is upscaled (the engine
// works best with text at least ~10 px tall), large images are scaled down
// to the engine's maximum size.
func prepare(img image.Image, maxDim uint32) ([]byte, int, int, float64) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	k := 1.0
	if mx := max(w, h); mx < 1600 {
		k = 2
	}
	if lim := float64(maxDim); float64(max(w, h))*k > lim {
		k = lim / float64(max(w, h))
	}
	ow, oh := max(1, int(float64(w)*k)), max(1, int(float64(h)*k))
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(src, src.Bounds(), image.White, image.Point{}, draw.Src) // transparent areas → white
	draw.Draw(src, src.Bounds(), img, b.Min, draw.Over)
	out := make([]byte, ow*oh*4)
	for y := 0; y < oh; y++ {
		sy := min(h-1, int(float64(y)/k))
		for x := 0; x < ow; x++ {
			sx := min(w-1, int(float64(x)/k))
			si, di := src.PixOffset(sx, sy), (y*ow+x)*4
			out[di], out[di+1], out[di+2], out[di+3] = src.Pix[si+2], src.Pix[si+1], src.Pix[si], 255
		}
	}
	return out, ow, oh, float64(ow) / float64(w)
}

func recognize(engine *win.COM, maxDim uint32, img image.Image) (Result, error) {
	b := img.Bounds()
	if b.Dx() < 4 || b.Dy() < 4 {
		return Result{}, nil
	}
	px, w, h, k := prepare(img, maxDim)

	cb, err := factory("Windows.Security.Cryptography.CryptographicBuffer", &iidCryptographicBufferStatics)
	if err != nil {
		return Result{}, err
	}
	defer cb.Release()
	var buf *win.COM
	if err := cb.HR("CreateFromByteArray", cbStaticsCreateFromByteArr, uintptr(len(px)), uintptr(unsafe.Pointer(&px[0])), uintptr(unsafe.Pointer(&buf))); err != nil {
		return Result{}, err
	}
	defer buf.Release()

	sb, err := factory("Windows.Graphics.Imaging.SoftwareBitmap", &iidSoftwareBitmapStatics)
	if err != nil {
		return Result{}, err
	}
	defer sb.Release()
	var bmp *win.COM
	if err := sb.HR("CreateCopyFromBuffer", sbStaticsCreateCopyFromBuf, uintptr(unsafe.Pointer(buf)), pixelFormatBgra8, uintptr(w), uintptr(h), uintptr(unsafe.Pointer(&bmp))); err != nil {
		return Result{}, err
	}
	defer bmp.Release()

	var op *win.COM
	if err := engine.HR("RecognizeAsync", ocrEngineRecognizeAsync, uintptr(unsafe.Pointer(bmp)), uintptr(unsafe.Pointer(&op))); err != nil {
		return Result{}, err
	}
	defer op.Release()
	var info *win.COM
	if err := op.HR("QueryInterface(IAsyncInfo)", 0, uintptr(unsafe.Pointer(&iidAsyncInfo)), uintptr(unsafe.Pointer(&info))); err != nil {
		return Result{}, err
	}
	defer info.Release()
	deadline := time.Now().Add(30 * time.Second)
	for {
		var st int32
		info.Call(asyncInfoStatus, uintptr(unsafe.Pointer(&st)))
		if st == asyncCompleted {
			break
		}
		if st != asyncStarted {
			var code int32
			info.Call(asyncInfoErrorCode, uintptr(unsafe.Pointer(&code)))
			return Result{}, fmt.Errorf("text recognition failed: HRESULT 0x%08X", uint32(code))
		}
		if time.Now().After(deadline) {
			return Result{}, errors.New("text recognition timed out")
		}
		time.Sleep(5 * time.Millisecond)
	}
	var res *win.COM
	if err := op.HR("GetResults", asyncOpGetResults, uintptr(unsafe.Pointer(&res))); err != nil {
		return Result{}, err
	}
	defer res.Release()

	var out Result
	var angleRef *win.COM
	if res.Call(ocrResultTextAngle, uintptr(unsafe.Pointer(&angleRef))) == 0 && angleRef != nil {
		angleRef.Call(ireferenceValue, uintptr(unsafe.Pointer(&out.Angle)))
		angleRef.Release()
	}
	var lines *win.COM
	if err := res.HR("Lines", ocrResultLines, uintptr(unsafe.Pointer(&lines))); err != nil {
		return Result{}, err
	}
	defer lines.Release()
	for _, line := range vector(lines) {
		var l Line
		var hs uintptr
		line.Call(ocrLineText, uintptr(unsafe.Pointer(&hs)))
		l.Text = goString(hs)
		pWindowsDeleteString.Call(hs)
		var words *win.COM
		if line.Call(ocrLineWords, uintptr(unsafe.Pointer(&words))) == 0 && words != nil {
			for _, wd := range vector(words) {
				var r struct{ X, Y, W, H float32 }
				wd.Call(ocrWordBoundingRect, uintptr(unsafe.Pointer(&r)))
				var ws uintptr
				wd.Call(ocrWordText, uintptr(unsafe.Pointer(&ws)))
				l.Words = append(l.Words, Word{Text: goString(ws), X: float64(r.X), Y: float64(r.Y), W: float64(r.W), H: float64(r.H)})
				pWindowsDeleteString.Call(ws)
				wd.Release()
			}
			words.Release()
		}
		line.Release()
		out.Lines = append(out.Lines, l)
	}
	out.Scale(k)
	return out, nil
}

// vector returns the items of an IVectorView (caller releases each).
func vector(v *win.COM) []*win.COM {
	var n uint32
	v.Call(vectorSize, uintptr(unsafe.Pointer(&n)))
	out := make([]*win.COM, 0, n)
	for i := uint32(0); i < n; i++ {
		var it *win.COM
		if v.Call(vectorGetAt, uintptr(i), uintptr(unsafe.Pointer(&it))) == 0 && it != nil {
			out = append(out, it)
		}
	}
	return out
}
