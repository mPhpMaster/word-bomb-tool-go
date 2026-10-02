//go:build windows

// Package winocr reads text with the OCR engine built into Windows 10/11
// (Windows.Media.Ocr). Launching tesseract.exe costs ~0.4-0.6s per call; the
// Windows engine stays loaded in this process and reads a short prompt in a few
// milliseconds.
//
// There is no WinRT projection for Go here, so this is a small hand-written
// client: activation factories come from combase.dll and methods are called
// through their vtables. Every WinRT call runs on one dedicated OS thread (a
// multithreaded apartment), which also serializes recognitions.
package winocr

import (
	"errors"
	"fmt"
	"image"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	combase                       = windows.NewLazySystemDLL("combase.dll")
	procRoInitialize              = combase.NewProc("RoInitialize")
	procRoGetActivationFactory    = combase.NewProc("RoGetActivationFactory")
	procWindowsCreateString       = combase.NewProc("WindowsCreateString")
	procWindowsDeleteString       = combase.NewProc("WindowsDeleteString")
	procWindowsGetStringRawBuffer = combase.NewProc("WindowsGetStringRawBuffer")
)

// Interface IDs (from the Windows SDK headers).
var (
	iidOcrEngineStatics      = windows.GUID{Data1: 0x5BFFA85A, Data2: 0x3384, Data3: 0x3540, Data4: [8]byte{0x99, 0x40, 0x69, 0x91, 0x20, 0xD4, 0x28, 0xA8}}
	iidSoftwareBitmapStatics = windows.GUID{Data1: 0xDF0385DB, Data2: 0x672F, Data3: 0x4A9D, Data4: [8]byte{0x80, 0x6E, 0xC2, 0x44, 0x2F, 0x34, 0x3E, 0x86}}
	iidBufferFactory         = windows.GUID{Data1: 0x71AF914D, Data2: 0xC10F, Data3: 0x484B, Data4: [8]byte{0xBC, 0x50, 0x14, 0xBC, 0x62, 0x3B, 0x3A, 0x27}}
	iidBufferByteAccess      = windows.GUID{Data1: 0x905A0FEF, Data2: 0xBC53, Data3: 0x11DF, Data4: [8]byte{0x8C, 0x49, 0x00, 0x1E, 0x4F, 0xC6, 0x86, 0xDA}}
	iidAsyncInfo             = windows.GUID{Data1: 0x00000036, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
)

// Vtable slots. Every WinRT interface starts with IUnknown (0-2) and
// IInspectable (3-5).
const (
	vtQueryInterface = 0
	vtRelease        = 2

	// IOcrEngineStatics
	vtMaxImageDimension                 = 6
	vtAvailableRecognizerLanguages      = 7
	vtTryCreateFromLanguage             = 9
	vtTryCreateFromUserProfileLanguages = 10
	// IOcrEngine
	vtRecognizeAsync     = 6
	vtRecognizerLanguage = 7
	// IOcrResult
	vtResultText = 8
	// ILanguage
	vtLanguageTag = 6
	// IVectorView<T>
	vtVectorGetAt = 6
	vtVectorSize  = 7
	// ISoftwareBitmapStatics
	vtCreateCopyFromBuffer = 9
	// IBufferFactory / IBuffer / IBufferByteAccess (IUnknown-based)
	vtBufferCreate     = 6
	vtBufferPutLength  = 8
	vtByteAccessBuffer = 3
	// IAsyncOperation<T> / IAsyncInfo
	vtAsyncGetResults = 8
	vtAsyncStatus     = 7
	vtAsyncErrorCode  = 8
)

const (
	pixelFormatGray8 = 62 // BitmapPixelFormat.Gray8

	asyncStarted   = 0
	asyncCompleted = 1

	recognizeTimeout = 5 * time.Second
)

// call invokes vtable slot idx of the COM object obj and returns its HRESULT.
func call(obj unsafe.Pointer, idx int, args ...uintptr) error {
	vtbl := *(*unsafe.Pointer)(obj)
	fn := *(*uintptr)(unsafe.Add(vtbl, idx*int(unsafe.Sizeof(uintptr(0)))))
	all := append([]uintptr{uintptr(obj)}, args...)
	hr, _, _ := syscall.SyscallN(fn, all...)
	return hresult(hr)
}

func hresult(hr uintptr) error {
	if int32(hr) < 0 {
		return fmt.Errorf("HRESULT 0x%08X", uint32(hr))
	}
	return nil
}

func release(obj unsafe.Pointer) {
	if obj != nil {
		_ = call(obj, vtRelease)
	}
}

func queryInterface(obj unsafe.Pointer, iid *windows.GUID) (unsafe.Pointer, error) {
	var out unsafe.Pointer
	if err := call(obj, vtQueryInterface, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out))); err != nil {
		return nil, err
	}
	return out, nil
}

// newHString creates an HSTRING; the caller deletes it with deleteHString.
func newHString(s string) (uintptr, error) {
	u, err := windows.UTF16FromString(s)
	if err != nil {
		return 0, err
	}
	var h uintptr
	hr, _, _ := procWindowsCreateString.Call(uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&h)))
	return h, hresult(hr)
}

func deleteHString(h uintptr) {
	if h != 0 {
		procWindowsDeleteString.Call(h)
	}
}

// hstringToString copies an HSTRING's text into a Go string.
func hstringToString(h uintptr) string {
	if h == 0 {
		return ""
	}
	var n uint32
	r, _, _ := procWindowsGetStringRawBuffer.Call(h, uintptr(unsafe.Pointer(&n)))
	if r == 0 || n == 0 {
		return ""
	}
	p := *(*unsafe.Pointer)(unsafe.Pointer(&r))
	return windows.UTF16ToString(unsafe.Slice((*uint16)(p), n))
}

func activationFactory(class string, iid *windows.GUID) (unsafe.Pointer, error) {
	h, err := newHString(class)
	if err != nil {
		return nil, err
	}
	defer deleteHString(h)
	var out unsafe.Pointer
	hr, _, _ := procRoGetActivationFactory.Call(h, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))
	if err := hresult(hr); err != nil {
		return nil, fmt.Errorf("%s: %w", class, err)
	}
	return out, nil
}

// languageTag reads ILanguage.LanguageTag.
func languageTag(lang unsafe.Pointer) string {
	var h uintptr
	if call(lang, vtLanguageTag, uintptr(unsafe.Pointer(&h))) != nil {
		return ""
	}
	defer deleteHString(h)
	return hstringToString(h)
}

// ---- worker thread ----------------------------------------------------------

type engine struct {
	ptr unsafe.Pointer // IOcrEngine, nil when the language isn't installed
	tag string
}

type worker struct {
	jobs chan func()

	// Owned by the worker thread.
	initErr       error
	ocrStatics    unsafe.Pointer
	bitmapStatics unsafe.Pointer
	bufferFactory unsafe.Pointer
	maxDim        uint32
	engines       map[string]*engine
}

var (
	once sync.Once
	w    *worker
)

func get() *worker {
	once.Do(func() {
		w = &worker{jobs: make(chan func()), engines: map[string]*engine{}}
		ready := make(chan struct{})
		go w.loop(ready)
		<-ready
	})
	return w
}

func (wk *worker) loop(ready chan struct{}) {
	// WinRT objects live in this thread's apartment, so keep the goroutine on it.
	runtime.LockOSThread()
	const roInitMultithreaded = 1
	hr, _, _ := procRoInitialize.Call(roInitMultithreaded)
	// S_FALSE (already initialised) is fine; RPC_E_CHANGED_MODE can't happen on a
	// fresh thread.
	if err := hresult(hr); err != nil {
		wk.initErr = fmt.Errorf("RoInitialize: %w", err)
	} else {
		wk.initErr = wk.initFactories()
	}
	close(ready)
	for job := range wk.jobs {
		job()
	}
}

func (wk *worker) initFactories() error {
	var err error
	if wk.ocrStatics, err = activationFactory("Windows.Media.Ocr.OcrEngine", &iidOcrEngineStatics); err != nil {
		return err
	}
	if wk.bitmapStatics, err = activationFactory("Windows.Graphics.Imaging.SoftwareBitmap", &iidSoftwareBitmapStatics); err != nil {
		return err
	}
	if wk.bufferFactory, err = activationFactory("Windows.Storage.Streams.Buffer", &iidBufferFactory); err != nil {
		return err
	}
	return call(wk.ocrStatics, vtMaxImageDimension, uintptr(unsafe.Pointer(&wk.maxDim)))
}

// run executes fn on the worker thread and waits for it.
func (wk *worker) run(fn func()) {
	done := make(chan struct{})
	wk.jobs <- func() {
		defer close(done)
		fn()
	}
	<-done
}

// engineFor returns the cached engine for a language prefix ("en", "ar"),
// creating it on first use. Worker thread only.
func (wk *worker) engineFor(lang string) *engine {
	if e, ok := wk.engines[lang]; ok {
		return e
	}
	e := &engine{}
	wk.engines[lang] = e
	if wk.initErr != nil {
		return e
	}

	var langs unsafe.Pointer // IVectorView<Language>
	if call(wk.ocrStatics, vtAvailableRecognizerLanguages, uintptr(unsafe.Pointer(&langs))) == nil && langs != nil {
		var n uint32
		_ = call(langs, vtVectorSize, uintptr(unsafe.Pointer(&n)))
		for i := uint32(0); i < n && e.ptr == nil; i++ {
			var l unsafe.Pointer
			if call(langs, vtVectorGetAt, uintptr(i), uintptr(unsafe.Pointer(&l))) != nil || l == nil {
				continue
			}
			if strings.HasPrefix(strings.ToLower(languageTag(l)), lang) {
				var eng unsafe.Pointer
				if call(wk.ocrStatics, vtTryCreateFromLanguage, uintptr(l), uintptr(unsafe.Pointer(&eng))) == nil && eng != nil {
					e.ptr = eng
				}
			}
			release(l)
		}
		release(langs)
	}
	if e.ptr == nil && lang == "en" {
		var eng unsafe.Pointer
		if call(wk.ocrStatics, vtTryCreateFromUserProfileLanguages, uintptr(unsafe.Pointer(&eng))) == nil && eng != nil {
			e.ptr = eng
		}
	}
	if e.ptr != nil {
		var l unsafe.Pointer
		if call(e.ptr, vtRecognizerLanguage, uintptr(unsafe.Pointer(&l))) == nil && l != nil {
			e.tag = languageTag(l)
			release(l)
		}
	}
	return e
}

// recognize runs the engine on an 8-bit grayscale image. Worker thread only.
func (wk *worker) recognize(e *engine, img *image.Gray) (string, error) {
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	size := uint32(width * height)

	// IBuffer holding the Gray8 pixels, rows packed without padding.
	var buf unsafe.Pointer
	if err := call(wk.bufferFactory, vtBufferCreate, uintptr(size), uintptr(unsafe.Pointer(&buf))); err != nil {
		return "", fmt.Errorf("Buffer.Create: %w", err)
	}
	defer release(buf)
	access, err := queryInterface(buf, &iidBufferByteAccess)
	if err != nil {
		return "", fmt.Errorf("IBufferByteAccess: %w", err)
	}
	var data unsafe.Pointer
	err = call(access, vtByteAccessBuffer, uintptr(unsafe.Pointer(&data)))
	if err == nil && data != nil {
		dst := unsafe.Slice((*byte)(data), size)
		for y := 0; y < height; y++ {
			off := (b.Min.Y+y-img.Rect.Min.Y)*img.Stride + (b.Min.X - img.Rect.Min.X)
			copy(dst[y*width:(y+1)*width], img.Pix[off:off+width])
		}
	}
	release(access)
	if err != nil || data == nil {
		return "", fmt.Errorf("IBufferByteAccess.Buffer: %v", err)
	}
	if err := call(buf, vtBufferPutLength, uintptr(size)); err != nil {
		return "", fmt.Errorf("IBuffer.Length: %w", err)
	}

	var bitmap unsafe.Pointer
	if err := call(wk.bitmapStatics, vtCreateCopyFromBuffer, uintptr(buf), pixelFormatGray8,
		uintptr(width), uintptr(height), uintptr(unsafe.Pointer(&bitmap))); err != nil {
		return "", fmt.Errorf("SoftwareBitmap.CreateCopyFromBuffer: %w", err)
	}
	defer release(bitmap)

	var op unsafe.Pointer // IAsyncOperation<OcrResult>
	if err := call(e.ptr, vtRecognizeAsync, uintptr(bitmap), uintptr(unsafe.Pointer(&op))); err != nil {
		return "", fmt.Errorf("RecognizeAsync: %w", err)
	}
	defer release(op)
	info, err := queryInterface(op, &iidAsyncInfo)
	if err != nil {
		return "", fmt.Errorf("IAsyncInfo: %w", err)
	}
	defer release(info)

	// Recognition takes a few milliseconds; poll rather than implementing a
	// COM completion handler.
	deadline := time.Now().Add(recognizeTimeout)
	var status int32
	for spins := 0; ; spins++ {
		if err := call(info, vtAsyncStatus, uintptr(unsafe.Pointer(&status))); err != nil {
			return "", fmt.Errorf("IAsyncInfo.Status: %w", err)
		}
		if status != asyncStarted {
			break
		}
		if time.Now().After(deadline) {
			return "", errors.New("recognition timed out")
		}
		if spins < 50 {
			runtime.Gosched()
		} else {
			time.Sleep(200 * time.Microsecond)
		}
	}
	if status != asyncCompleted {
		var code uintptr
		_ = call(info, vtAsyncErrorCode, uintptr(unsafe.Pointer(&code)))
		return "", fmt.Errorf("recognition failed (status %d, %v)", status, hresult(code))
	}

	var result unsafe.Pointer // IOcrResult
	if err := call(op, vtAsyncGetResults, uintptr(unsafe.Pointer(&result))); err != nil {
		return "", fmt.Errorf("GetResults: %w", err)
	}
	defer release(result)
	var text uintptr
	if err := call(result, vtResultText, uintptr(unsafe.Pointer(&text))); err != nil {
		return "", fmt.Errorf("OcrResult.Text: %w", err)
	}
	defer deleteHString(text)
	return hstringToString(text), nil
}

// ---- public API ---------------------------------------------------------------

// Available reports whether an OCR engine for the language prefix ("en", "ar")
// is installed. The engine is created (and kept) on first use.
func Available(lang string) bool {
	return LanguageTag(lang) != ""
}

// LanguageTag returns the recognizer language used for the prefix (such as
// "en-US"), or "" when that OCR language isn't installed.
func LanguageTag(lang string) string {
	wk := get()
	var tag string
	wk.run(func() {
		if e := wk.engineFor(lang); e.ptr != nil {
			tag = e.tag
		}
	})
	return tag
}

// InitError returns why WinRT OCR could not start, or nil.
func InitError() error {
	return get().initErr
}

// Recognize reads the text in an 8-bit grayscale image with the engine for the
// language prefix. Calls are serialized.
func Recognize(img *image.Gray, lang string) (string, error) {
	if img == nil || img.Bounds().Dx() <= 0 || img.Bounds().Dy() <= 0 {
		return "", errors.New("empty image")
	}
	wk := get()
	var (
		text string
		err  error
	)
	wk.run(func() {
		e := wk.engineFor(lang)
		if e.ptr == nil {
			err = fmt.Errorf("no %q OCR language installed", lang)
			if wk.initErr != nil {
				err = wk.initErr
			}
			return
		}
		// The engine rejects images larger than MaxImageDimension on either side.
		if uint32(img.Bounds().Dx()) > wk.maxDim || uint32(img.Bounds().Dy()) > wk.maxDim {
			err = fmt.Errorf("image %dx%d exceeds the OCR limit of %d", img.Bounds().Dx(), img.Bounds().Dy(), wk.maxDim)
			return
		}
		text, err = wk.recognize(e, img)
	})
	return text, err
}
